// Package webhook — issue-10.2 inbound webhooks.
//
// Cliente externo (GitHub, GitLab, generic) hace POST /webhooks/:slug?token=...
// Domain verifica HMAC y dispatchea target (flow/agent/skill).
//
// Secret se cifra at-rest con crypto.AESGCM (issue-02.3). Cada delivery se
// persiste en webhook_deliveries para auditoría.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/crypto"
	"nunezlagos/domain/internal/service/webhook/webhookdb"
	"nunezlagos/domain/internal/store/txctx"
)

var (
	ErrSlugInvalid       = errors.New("slug must be lowercase ascii, digits, dashes")
	ErrSlugTaken         = errors.New("slug already taken")
	ErrInvalidSourceType = errors.New("source_type must be generic|github|gitlab")
	ErrInvalidTargetType = errors.New("target_type must be flow|agent|skill")
	ErrNotFound          = errors.New("webhook not found")
	ErrProjectRequired   = errors.New("project_id required: es el eje de RLS de webhooks (000288)")
	ErrSignatureInvalid  = errors.New("HMAC signature invalid")
)

var (
	reSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)
	// bitbucket salió de la lista en DOMAINSERV-240: verifyWebhookSignature devolvía
	// true sin mirar nada para ese source_type, así que un webhook bitbucket aceptaba
	// cualquier payload de cualquiera. Un source_type sin verificación de firma
	// implementada no se puede dar de alta. El CHECK de la migración 000017 sigue
	// aceptándolo —una migración aplicada no se edita— y esta lista es más estricta,
	// que es la dirección segura.
	validSources = map[string]bool{"generic": true, "github": true, "gitlab": true}
	validTargets = map[string]bool{"flow": true, "agent": true, "skill": true}
)

type Webhook struct {
	ID             uuid.UUID
	ProjectID      *uuid.UUID
	Slug           string
	Name           string
	SourceType     string
	TargetType     string
	TargetID       uuid.UUID
	InputsMapping  map[string]any
	Enabled        bool
	LastDeliveryAt *time.Time
}

type CreateInput struct {
	OrganizationID uuid.UUID
	// ProjectID es el eje de RLS de webhooks desde la 000288 y es OBLIGATORIO: sin él la
	// fila viola WITH CHECK (project_id = current_project_id()) y el alta falla. Una fila
	// con project_id NULL además quedaría invisible para el management y la migración la
	// deja enabled=false, o sea un endpoint que nadie puede listar ni borrar.
	ProjectID uuid.UUID
	CreatedBy *uuid.UUID
	Slug      string
	Name           string
	Secret         string // plaintext, se cifra antes de persistir
	SourceType     string
	TargetType     string
	TargetID       uuid.UUID
	InputsMapping  map[string]any
	ActorID        uuid.UUID
}

type Service struct {
	Pool   *pgxpool.Pool
	Audit  audit.Recorder
	Crypto *crypto.Cipher // para cifrar secret at-rest
	// PoolPublic es el pool de app_admin (BYPASSRLS) y lo usa SOLO el camino de recepción:
	// ResolveBySlug y RecordDelivery.
	//
	// POR QUÉ hace falta (DOMAINSERV-240): desde la 000288 webhooks está bajo RLS por
	// app.current_project_id, pero /receive es un endpoint PÚBLICO que solo conoce el slug
	// — no tiene proyecto en el contexto del request, así que no hay GUC que setear. Con el
	// pool de la app, GetWebhookBySlug devolvería CERO filas sin error y el endpoint daría
	// 401 para todo slug: la feature seguiría inalcanzable, ahora por RLS en vez de por
	// falta de alta. La resolución global es correcta porque el slug es único en toda la
	// instancia (índice webhooks_slug_global_uniq de la misma migración).
	//
	// El aislamiento por proyecto NO se pierde: el management (List, Deliveries, GetByID)
	// sigue pasando por Pool bajo RLS, que es donde alguien podría leer datos de otro
	// proyecto. Si queda en nil se cae a Pool, que es lo correcto en dev sin RLS.
	PoolPublic *pgxpool.Pool
}

func (s *Service) q(ctx context.Context) *webhookdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return webhookdb.New(tx)
	}
	return webhookdb.New(s.Pool)
}

// qPublic es el acceso del camino de recepción: sin GUC de proyecto y por eso sobre el pool
// que no filtra. Ignora la tx del contexto a propósito — la tx del request de /receive no
// tiene el GUC seteado, así que usarla reintroduciría el filtro que este camino no puede
// satisfacer.
func (s *Service) qPublic() *webhookdb.Queries {
	if s.PoolPublic != nil {
		return webhookdb.New(s.PoolPublic)
	}
	return webhookdb.New(s.Pool)
}

// webhookRow agrupa las filas generadas por sqlc para las queries que
// devuelven un webhook completo. Tras de-orgear, sqlc ya no reusa un unico
// struct de tabla (organization_id fue dropeado de la BD pero sigue en el
// schema estatico que sqlc lee), asi que cada query emite su propio Row
// estructuralmente identico. El union permite un solo conversor.
type webhookRow interface {
	webhookdb.GetWebhookByIDRow |
		webhookdb.GetWebhookBySlugRow |
		webhookdb.InsertWebhookRow |
		webhookdb.ListWebhooksRow
}

func webhookFromRow[T webhookRow](src T) Webhook {
	row := webhookdb.GetWebhookByIDRow(src)
	w := Webhook{
		ID:         row.ID,
		ProjectID:  row.ProjectID,
		Slug:       row.Slug,
		Name:       row.Name,
		SourceType: row.SourceType,
		TargetType: row.TargetType,
		TargetID:   row.TargetID,
		Enabled:    row.Enabled,
	}
	if len(row.InputsMapping) > 0 {
		_ = json.Unmarshal(row.InputsMapping, &w.InputsMapping)
	}
	if w.InputsMapping == nil {
		w.InputsMapping = map[string]any{}
	}
	if row.LastDeliveryAt.Valid {
		w.LastDeliveryAt = &row.LastDeliveryAt.Time
	}
	return w
}

func unmarshalMap(b []byte) map[string]any {
	var m map[string]any
	if len(b) > 0 {
		_ = json.Unmarshal(b, &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Webhook, error) {
	if !reSlug.MatchString(in.Slug) {
		return nil, ErrSlugInvalid
	}
	if in.ProjectID == uuid.Nil {
		return nil, ErrProjectRequired
	}
	if !validSources[in.SourceType] {
		return nil, ErrInvalidSourceType
	}
	if !validTargets[in.TargetType] {
		return nil, ErrInvalidTargetType
	}
	if in.Secret == "" {
		return nil, errors.New("secret required")
	}

	encSecret, err := s.Crypto.Encrypt([]byte(in.Secret))
	if err != nil {
		return nil, fmt.Errorf("encrypt secret: %w", err)
	}
	if in.InputsMapping == nil {
		in.InputsMapping = map[string]any{}
	}
	mappingJSON, _ := json.Marshal(in.InputsMapping)

	projectID := in.ProjectID
	row, err := s.q(ctx).InsertWebhook(ctx, webhookdb.InsertWebhookParams{
		CreatedBy:       in.CreatedBy,
		ProjectID:       &projectID,
		Slug:            in.Slug,
		Name:            in.Name,
		SecretEncrypted: encSecret,
		SourceType:      in.SourceType,
		TargetType:      in.TargetType,
		TargetID:        in.TargetID,
		InputsMapping:   mappingJSON,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert webhook: %w", err)
	}
	w := webhookFromRow(row)
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "webhook.created",
			EntityType:     "webhook",
			EntityID:       &w.ID,
			NewValues:      map[string]any{"slug": w.Slug, "target_type": w.TargetType},
		})
	}
	return &w, nil
}

// ResolveBySlug busca webhook + descifra secret para verificar HMAC. Va por qPublic: el
// endpoint que la llama es público y solo conoce el slug, así que no hay proyecto con el que
// satisfacer el RLS de la 000288 (ver el comentario de PoolPublic).
func (s *Service) ResolveBySlug(ctx context.Context, slug string) (*Webhook, []byte, error) {
	row, err := s.qPublic().GetWebhookBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query webhook: %w", err)
	}
	if !row.Enabled {
		return nil, nil, ErrNotFound
	}
	secret, err := s.Crypto.Decrypt(row.SecretEncrypted)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt secret: %w", err)
	}
	w := webhookFromRow(row)
	return &w, secret, nil
}

// VerifyHMAC verifica una signature HMAC-SHA256 sobre el body.
// signatureHex viene del header (e.g. X-Hub-Signature-256 de GitHub).
func VerifyHMAC(secret, body []byte, signatureHex string) bool {

	sig := strings.TrimPrefix(signatureHex, "sha256=")
	expected, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	actual := mac.Sum(nil)
	return hmac.Equal(expected, actual)
}

// RecordDelivery persiste un webhook_delivery (status + run_id + error).
func (s *Service) RecordDelivery(ctx context.Context, webhookID uuid.UUID,
	payload []byte, headers map[string]string, sourceIP, status string,
	triggeredRunID *uuid.UUID, errStr string) error {
	headersJSON, _ := json.Marshal(headers)
	// payload es JSONB NOT NULL: un slice vacío llega como NULL y el INSERT falla. Como
	// todos los llamadores ignoran el error de RecordDelivery, ese fallo se traduciría en
	// una entrega que no queda registrada, en silencio. El caso vacío es legítimo desde
	// DOMAINSERV-240: la firma inválida se registra sin el cuerpo.
	if len(payload) == 0 {
		payload = []byte("{}")
	}

	// qPublic por lo mismo que ResolveBySlug: la escritura ocurre en el camino de recepción,
	// que no tiene proyecto en el contexto. webhook_deliveries hereda su scope del webhook
	// padre vía webhook_id, así que la fila no queda huérfana de proyecto.
	err := s.qPublic().InsertDelivery(ctx, webhookdb.InsertDeliveryParams{
		WebhookID:      webhookID,
		Payload:        payload,
		Headers:        headersJSON,
		SourceIp:       strPtr(sourceIP),
		Status:         status,
		Error:          strPtr(errStr),
		TriggeredRunID: triggeredRunID,
	})
	if err != nil {
		return fmt.Errorf("record delivery: %w", err)
	}
	_ = s.qPublic().UpdateLastDelivery(ctx, webhookID)
	return nil
}
