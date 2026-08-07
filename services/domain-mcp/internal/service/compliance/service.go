// Package compliance — marcos normativos por proyecto con crosswalk de controles
// (issue-56.4).
//
// Dos mitades con reglas de acceso distintas, y esa división es el diseño:
//
//   - CATÁLOGO (compliance_frameworks, compliance_controls, compliance_framework_controls): global a la
//     instancia, sin RLS. Qué ES la Ley 21.719 no depende del proyecto que la mire.
//   - POR PROYECTO (project_compliance_frameworks, project_control_status): bajo RLS por
//     app.current_project_id. A qué está afecto un proyecto es información suya.
//
// El opt-in es real: la AUSENCIA de fila en project_compliance_frameworks significa que el marco
// no aplica. Es lo contrario del modelo de skills, donde una skill global auto-aplica a todos los
// proyectos y project_skills solo sirve para excluir.
package compliance

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/store/txctx"
)

var (
	ErrMarcoNoEncontrado   = errors.New("marco normativo no encontrado")
	ErrControlNoEncontrado = errors.New("control no encontrado en el catálogo")
	ErrEstadoInvalido      = errors.New("estado debe ser ok|parcial|falta|no_verificable")
	// ErrFuenteNoRedistribuible protege de ingestar el texto de una norma de pago. Las leyes son
	// públicas; el articulado de ISO/IEC no se puede redistribuir ni en un repo privado.
	ErrFuenteNoRedistribuible = errors.New(
		"marco de fuente no redistribuible: guardá referencia de cláusula y evidencia, nunca el texto")
)

var estadosValidos = map[string]bool{
	"ok": true, "parcial": true, "falta": true, "no_verificable": true,
}

// Framework es una entrada del catálogo: una ley, un reglamento o una norma técnica.
type Framework struct {
	ID           uuid.UUID  `json:"id"`
	Slug         string     `json:"slug"`
	Nombre       string     `json:"nombre"`
	Tipo         string     `json:"tipo"`
	Jurisdiccion string     `json:"jurisdiccion,omitempty"`
	Obligatorio  bool       `json:"obligatorio"`
	Certificable bool       `json:"certificable"`
	Edicion      string     `json:"edicion,omitempty"`
	VigenteDesde *time.Time `json:"vigente_desde,omitempty"`
	FuenteTipo   string     `json:"fuente_tipo"`
}

// Vigente responde si el marco ya rige a la fecha dada. Un marco sin vigente_desde se considera
// vigente: es el caso de las normas técnicas, que no tienen fecha de entrada en vigor.
//
// Importa para la severidad: la Ley 21.719 rige recién desde 2026-12-01, así que incumplirla hoy
// no es lo mismo que incumplirla el año que viene.
func (f Framework) Vigente(ahora time.Time) bool {
	if f.VigenteDesde == nil {
		return true
	}
	return !ahora.Before(*f.VigenteDesde)
}

// AdmiteTextoCompleto responde si el articulado se puede ingestar a knowledge. Es el guard de
// copyright: las leyes chilenas y el GDPR son públicos, las normas ISO/IEC son de pago.
func (f Framework) AdmiteTextoCompleto() bool {
	return f.FuenteTipo == "texto_libre"
}

// ControlExigido es un control con la cita del marco que lo pide. El mismo control aparece una vez
// por cada marco que lo exige, con su propia referencia — eso es el crosswalk.
type ControlExigido struct {
	ControlID     uuid.UUID `json:"control_id"`
	ControlSlug   string    `json:"control_slug"`
	Nombre        string    `json:"nombre"`
	FrameworkSlug string    `json:"framework_slug"`
	Referencia    string    `json:"referencia"`
	Obligatorio   bool      `json:"obligatorio"`
	Vigente       bool      `json:"vigente"`
}

// consultor es la interfaz chica que este service consume. Pool y Tx la satisfacen igual, y se
// define acá —en el consumidor— y no junto a la implementación.
type consultor interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Service struct {
	Pool *pgxpool.Pool
}

// q usa la tx del contexto cuando existe: las escrituras por proyecto necesitan el GUC de RLS que
// el wireup dejó seteado en esa tx. El catálogo no lo necesita, pero usar la misma tx mantiene la
// lectura consistente dentro de un request.
func (s *Service) q(ctx context.Context) consultor {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return s.Pool
}
