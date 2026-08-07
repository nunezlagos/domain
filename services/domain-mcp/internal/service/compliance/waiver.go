package compliance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// razonMinima es el largo que separa una razón de un trámite. Diez caracteres no garantizan
// calidad, pero descartan el "ok" y el "-" que convierten al waiver en un botón de saltear.
const razonMinima = 10

// ErrRazonRequerida se devuelve cuando el waiver no trae una razón utilizable. Un waiver sin razón
// escrita es un bypass con otro nombre, y la razón es justamente lo que hace auditable la decisión.
var ErrRazonRequerida = fmt.Errorf(
	"el waiver exige una razón escrita de al menos %d caracteres: sin ella no es auditable",
	razonMinima)

// Waiver es una excepción otorgada sobre una obligación concreta.
type Waiver struct {
	ID            uuid.UUID  `json:"id"`
	ControlSlug   string     `json:"control_slug"`
	FrameworkSlug string     `json:"framework_slug"`
	Razon         string     `json:"razon"`
	OtorgadoPor   *uuid.UUID `json:"otorgado_por_id,omitempty"`
	OtorgadoAt    time.Time  `json:"otorgado_at"`
	VenceAt       *time.Time `json:"vence_at,omitempty"`
}

// Vigente responde si el waiver todavía destraba a la fecha dada. Uno vencido no destraba nada:
// la excepción caduca, la obligación no.
func (w Waiver) Vigente(ahora time.Time) bool {
	if w.VenceAt == nil {
		return true
	}
	return ahora.Before(*w.VenceAt)
}

// OtorgarWaiver registra la excepción. Es idempotente por (proyecto, control, marco): re-otorgar
// actualiza la razón en vez de acumular filas, porque el caso real es corregir el texto.
func (s *Service) OtorgarWaiver(ctx context.Context, projectID uuid.UUID,
	controlSlug, frameworkSlug, razon string, actorID *uuid.UUID, venceAt *time.Time,
	flowRunID *uuid.UUID,
) (uuid.UUID, error) {
	if len(strings.TrimSpace(razon)) < razonMinima {
		return uuid.Nil, ErrRazonRequerida
	}
	if controlSlug == "" || frameworkSlug == "" {
		return uuid.Nil, errors.New("waiver: control_slug y framework_slug son requeridos")
	}
	var id uuid.UUID
	err := s.q(ctx).QueryRow(ctx,
		`INSERT INTO compliance_waivers
		     (project_id, control_slug, framework_slug, razon, otorgado_por_id, vence_at, flow_run_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (project_id, control_slug, framework_slug) WHERE revocado_at IS NULL
		 DO UPDATE SET razon = EXCLUDED.razon, otorgado_por_id = EXCLUDED.otorgado_por_id,
		               vence_at = EXCLUDED.vence_at, otorgado_at = NOW()
		 RETURNING id`,
		projectID, controlSlug, frameworkSlug, strings.TrimSpace(razon), actorID, venceAt, flowRunID).
		Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("otorgar waiver: %w", err)
	}
	return id, nil
}

// WaiversVigentes devuelve los waivers vivos del proyecto a la fecha dada. Los vencidos NO se
// devuelven: dejarlos pasar convertiría una excepción temporal en permanente sin que nadie lo note.
func (s *Service) WaiversVigentes(ctx context.Context, projectID uuid.UUID, ahora time.Time,
) ([]Waiver, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT id, control_slug, framework_slug, razon, otorgado_por_id, otorgado_at, vence_at
		 FROM compliance_waivers
		 WHERE project_id = $1 AND revocado_at IS NULL
		   AND (vence_at IS NULL OR vence_at > $2)
		 ORDER BY otorgado_at DESC`, projectID, ahora)
	if err != nil {
		return nil, fmt.Errorf("waivers vigentes: %w", err)
	}
	defer rows.Close()
	var out []Waiver
	for rows.Next() {
		var w Waiver
		if err := rows.Scan(&w.ID, &w.ControlSlug, &w.FrameworkSlug, &w.Razon,
			&w.OtorgadoPor, &w.OtorgadoAt, &w.VenceAt); err != nil {
			return nil, fmt.Errorf("scan waiver: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// RevocarWaiver deja sin efecto una excepción. No borra la fila: el registro de que se otorgó y
// por qué es justamente lo que hay que conservar para una auditoría.
func (s *Service) RevocarWaiver(ctx context.Context, projectID, waiverID uuid.UUID) error {
	tag, err := s.q(ctx).Exec(ctx,
		`UPDATE compliance_waivers SET revocado_at = NOW()
		 WHERE id = $1 AND project_id = $2 AND revocado_at IS NULL`, waiverID, projectID)
	if err != nil {
		return fmt.Errorf("revocar waiver: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("waiver: no encontrado o ya revocado")
	}
	return nil
}
