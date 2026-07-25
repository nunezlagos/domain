package apikey

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// signedCandidatesSQL trae los candidatos por prefix con su plaintext
// descifrado. key_ciphertext lo pobló la migración 000168 con
// pgp_sym_encrypt(plaintext, DOMAIN_FIELD_ENC_KEY), que SÍ es reversible: de ahí
// sale el secreto de firma sin necesidad de columna nueva. El CASE evita que
// pgp_sym_decrypt reciba NULL en las keys pre-000168.
const signedCandidatesSQL = `
	SELECT k.id, k.user_id, COALESCE(u.role,'viewer'),
	       CASE WHEN k.key_ciphertext IS NULL THEN NULL
	            ELSE pgp_sym_decrypt(k.key_ciphertext, $2)::text END
	  FROM auth_api_keys k
	  JOIN users u ON u.id = k.user_id
	 WHERE k.key_prefix = $1
	   AND k.revoked_at IS NULL
	   AND (k.expires_at IS NULL OR k.expires_at > NOW())
	   AND u.deleted_at IS NULL`

// ResolveSigned implementa SignedResolver: recompone el secreto de cada
// candidato del prefix y devuelve el Principal del que reproduce la firma.
func (s *PGStore) ResolveSigned(ctx context.Context, keyPrefix, canonical, sigHex string) (*Principal, error) {
	if s.FieldEncKey == "" {
		return nil, ErrNoEncKey
	}
	if _, err := ParsePrefix(keyPrefix); err != nil {
		return nil, ErrNotFound
	}
	rows, err := s.Pool.Query(ctx, signedCandidatesSQL, keyPrefix, s.FieldEncKey)
	if err != nil {
		return nil, fmt.Errorf("query signed candidates: %w", err)
	}
	defer rows.Close()
	return s.matchSignedCandidate(rows, canonical, sigHex)
}

// matchSignedCandidate itera los candidatos: el prefix NO es único (PrefixLen=16
// y "domk_live_" ya come 10 chars), así que hay que probarlos todos igual que
// Resolve.
func (s *PGStore) matchSignedCandidate(rows pgx.Rows, canonical, sigHex string) (*Principal, error) {
	// las keys emitidas antes de 000168 tienen key_ciphertext NULL y no pueden
	// firmar nunca: bcrypt no se invierte, no hay backfill posible, hay que rotarlas
	unsignable := false
	for rows.Next() {
		var (
			id, userID uuid.UUID
			role       string
			plaintext  *string
		)
		if err := rows.Scan(&id, &userID, &role, &plaintext); err != nil {
			return nil, fmt.Errorf("scan signed candidate: %w", err)
		}
		if plaintext == nil {
			unsignable = true
			continue
		}
		if !SignatureMatches(SigningSecret(*plaintext), canonical, sigHex) {
			continue
		}
		s.touchLastUsed(id)
		return &Principal{
			UserID:         userID.String(),
			OrganizationID: canonicalOrgID.String(),
			APIKeyID:       id.String(),
			Role:           role,
		}, nil
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate signed candidates: %w", err)
	}
	if unsignable {
		return nil, ErrKeyNotSignable
	}
	return nil, ErrNotFound
}

// nonceRetention margen sobre SignatureMaxSkew antes de poder un nonce: fuera de
// la ventana la firma ya se rechaza por ts, así que el nonce deja de ser útil.
const nonceRetention = "15 minutes"

// nonceBurnBatch tope de filas viejas que se podan por request. Acotado para
// que el costo del quemado no dependa del backlog.
const nonceBurnBatch = "100"

// burnNonceSQL quema el nonce y poda los vencidos en el mismo statement.
//
// El INSERT ... ON CONFLICT DO NOTHING sobre la PK (api_key_id, nonce) es la
// primitiva atómica: dos requests concurrentes con el mismo nonce y solo una
// inserta fila. RowsAffected()==0 ⇒ replay.
//
// La poda va en un CTE porque no hay cron que limpie la tabla y sin eso crece
// sin techo. SKIP LOCKED + LIMIT la vuelven no-bloqueante y de costo fijo: si
// otro request ya está podando esas filas, este las saltea en vez de esperar.
const burnNonceSQL = `
	WITH stale AS (
	  SELECT api_key_id, nonce
	    FROM auth_hmac_nonces
	   WHERE signed_at < NOW() - INTERVAL '` + nonceRetention + `'
	   ORDER BY signed_at
	   LIMIT ` + nonceBurnBatch + `
	     FOR UPDATE SKIP LOCKED
	), pruned AS (
	  DELETE FROM auth_hmac_nonces n
	   USING stale s
	   WHERE n.api_key_id = s.api_key_id AND n.nonce = s.nonce
	)
	INSERT INTO auth_hmac_nonces (api_key_id, nonce, signed_at)
	VALUES ($1, $2, $3)
	ON CONFLICT (api_key_id, nonce) DO NOTHING`

// BurnNonce implementa NonceBurner. fresh=false significa replay: ese par
// (key, nonce) ya se había quemado.
func (s *PGStore) BurnNonce(ctx context.Context, apiKeyID, nonce string, signedAt time.Time) (bool, error) {
	keyID, err := uuid.Parse(apiKeyID)
	if err != nil {
		return false, fmt.Errorf("parse api key id: %w", err)
	}
	tag, err := s.Pool.Exec(ctx, burnNonceSQL, keyID, nonce, signedAt)
	if err != nil {
		return false, fmt.Errorf("burn nonce: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
