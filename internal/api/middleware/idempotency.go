// Package middleware — HU-13.4 idempotency keys.
//
// Cliente envía header Idempotency-Key: <uuid|nanoid>. Si la key ya existe
// para esta org y request_body_hash coincide → devuelve la response cached.
// Si key existe pero body distinto → 409 conflict (mismatch).
//
// Solo se aplica a POST/PATCH/DELETE (mutations). GET no necesita
// idempotency (ya es safe).
package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/auth/apikey"
)

const HeaderIdempotencyKey = "Idempotency-Key"
const HeaderReplayed = "Idempotent-Replayed"

type Idempotency struct {
	Pool *pgxpool.Pool
}

// Wrap aplica idempotency check para POST/PATCH/DELETE. Skip si no hay key
// o request es GET/HEAD/OPTIONS.
func (i *Idempotency) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !shouldApply(r) {
			next.ServeHTTP(w, r)
			return
		}
		key := r.Header.Get(HeaderIdempotencyKey)
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		p, ok := apikey.FromContext(r.Context())
		if !ok || p == nil {
			next.ServeHTTP(w, r)
			return
		}
		orgID, err := uuid.Parse(p.OrganizationID)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Leer + cachear body (para re-pasar al handler downstream)
		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		bodyHash := sha256.Sum256(bodyBytes)

		// Lookup
		cached, hashStored, err := i.lookup(r.Context(), orgID, key)
		if err == nil && cached != nil {
			// Existe — verificar hash
			if !bytes.Equal(hashStored, bodyHash[:]) {
				writeError(w, http.StatusConflict, "idempotency_mismatch",
					"Idempotency-Key reused with different request body")
				return
			}
			// Match → devolver response cacheada
			replayResponse(w, cached)
			return
		}

		// Captura del response para guardar
		rec := &responseRecorder{ResponseWriter: w, status: 200, body: &bytes.Buffer{}}
		next.ServeHTTP(rec, r)

		// Solo cachear 2xx / 4xx (no 5xx — server errors no son determinísticos)
		if rec.status < 500 {
			_ = i.store(r.Context(), orgID, p, key, r, bodyHash[:], rec)
		}
	})
}

func shouldApply(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

type cachedResponse struct {
	Status  int
	Headers map[string][]string
	Body    []byte
}

func (i *Idempotency) lookup(ctx context.Context, orgID uuid.UUID, key string) (*cachedResponse, []byte, error) {
	var (
		status  int16
		headers []byte
		body    []byte
		hash    []byte
	)
	err := i.Pool.QueryRow(ctx,
		`SELECT response_status, response_headers, response_body, request_body_hash
		 FROM idempotency_keys
		 WHERE organization_id = $1 AND key = $2 AND expires_at > NOW()`,
		orgID, key,
	).Scan(&status, &headers, &body, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, err
	}
	if err != nil {
		return nil, nil, err
	}
	cr := &cachedResponse{Status: int(status), Body: body}
	if len(headers) > 0 {
		_ = json.Unmarshal(headers, &cr.Headers)
	}
	return cr, hash, nil
}

func (i *Idempotency) store(ctx context.Context, orgID uuid.UUID, p *apikey.Principal,
	key string, r *http.Request, bodyHash []byte, rec *responseRecorder) error {
	headerJSON, _ := json.Marshal(rec.Header())
	var userID *uuid.UUID
	if uid, err := uuid.Parse(p.UserID); err == nil {
		userID = &uid
	}
	_, err := i.Pool.Exec(ctx,
		`INSERT INTO idempotency_keys
		   (organization_id, user_id, key, request_method, request_path,
		    request_body_hash, response_status, response_headers, response_body)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (organization_id, key) DO NOTHING`,
		orgID, userID, key, r.Method, r.URL.Path,
		bodyHash, int16(rec.status), headerJSON, rec.body.Bytes(),
	)
	return err
}

func replayResponse(w http.ResponseWriter, c *cachedResponse) {
	for k, vs := range c.Headers {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set(HeaderReplayed, "true")
	w.WriteHeader(c.Status)
	_, _ = w.Write(c.Body)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + msg + `"}}`))
}

// responseRecorder captura status + body para cachear.
type responseRecorder struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(s int) {
	r.status = s
	r.ResponseWriter.WriteHeader(s)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// CleanupExpired job: drop keys vencidas (cron diario).
func (i *Idempotency) CleanupExpired(ctx context.Context) (int64, error) {
	tag, err := i.Pool.Exec(ctx,
		`DELETE FROM idempotency_keys WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ensure time import
var _ = time.Now
