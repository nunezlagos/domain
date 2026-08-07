package compliance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// las dos listas describen las MISMAS columnas en el mismo orden; la segunda va con alias para las
// queries con JOIN. Cambiar una sin la otra rompe el Scan, que es por qué están pegadas acá.
const columnasFramework = `id, slug, nombre, tipo, COALESCE(jurisdiccion,''), obligatorio,
	certificable, edicion, vigente_desde, fuente_tipo`

const columnasFrameworkCF = `cf.id, cf.slug, cf.nombre, cf.tipo, COALESCE(cf.jurisdiccion,''),
	cf.obligatorio, cf.certificable, cf.edicion, cf.vigente_desde, cf.fuente_tipo`

func escanearFrameworks(rows pgx.Rows) ([]Framework, error) {
	defer rows.Close()
	var out []Framework
	for rows.Next() {
		var f Framework
		if err := rows.Scan(&f.ID, &f.Slug, &f.Nombre, &f.Tipo, &f.Jurisdiccion,
			&f.Obligatorio, &f.Certificable, &f.Edicion, &f.VigenteDesde, &f.FuenteTipo); err != nil {
			return nil, fmt.Errorf("scan framework: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ListarCatalogo devuelve todos los marcos conocidos por la instancia. NO necesita scope de
// proyecto: el catálogo es global y está fuera de RLS a propósito.
func (s *Service) ListarCatalogo(ctx context.Context) ([]Framework, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT `+columnasFramework+` FROM compliance_frameworks
		 WHERE deleted_at IS NULL ORDER BY slug, edicion`)
	if err != nil {
		return nil, fmt.Errorf("listar catálogo: %w", err)
	}
	return escanearFrameworks(rows)
}

// MarcosDelProyecto devuelve los marcos que el proyecto DECLARÓ. Un proyecto que no declaró nada
// devuelve vacío, y eso significa que no está afecto a ninguno: es el opt-in.
func (s *Service) MarcosDelProyecto(ctx context.Context, projectID uuid.UUID) ([]Framework, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT `+columnasFrameworkCF+` FROM compliance_frameworks cf
		 JOIN project_compliance_frameworks pcf ON pcf.framework_id = cf.id
		 WHERE pcf.project_id = $1 AND pcf.activo AND cf.deleted_at IS NULL
		 ORDER BY cf.slug`, projectID)
	if err != nil {
		return nil, fmt.Errorf("marcos del proyecto: %w", err)
	}
	return escanearFrameworks(rows)
}

// DeclararMarco activa un marco para el proyecto. Es idempotente: re-declarar el mismo marco lo
// reactiva en vez de fallar, porque el caso real es corregir una desactivación.
func (s *Service) DeclararMarco(ctx context.Context, projectID, frameworkID uuid.UUID,
	actorID *uuid.UUID, activo bool,
) error {
	tag, err := s.q(ctx).Exec(ctx,
		`INSERT INTO project_compliance_frameworks (project_id, framework_id, activo, activado_por_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (project_id, framework_id)
		 DO UPDATE SET activo = EXCLUDED.activo, activado_por_id = EXCLUDED.activado_por_id,
		               activado_at = NOW()`,
		projectID, frameworkID, activo, actorID)
	if err != nil {
		return fmt.Errorf("declarar marco: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMarcoNoEncontrado
	}
	return nil
}

// ControlesExigidos expande el crosswalk: devuelve los controles que exigen los marcos declarados
// por el proyecto, cada uno con la referencia del marco que lo pide.
//
// Un mismo control aparece una vez por marco: "cifrado-en-reposo" sale con 'Art. 32' para gdpr y
// con su cláusula para iso-27001. Evaluarlo una vez y reportarlo en todos es el objetivo.
//
// Sin N+1: una sola query con dos JOIN, no una consulta por marco.
func (s *Service) ControlesExigidos(ctx context.Context, projectID uuid.UUID, ahora time.Time,
) ([]ControlExigido, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT cc.id, cc.slug, cc.nombre, cf.slug, fc.referencia, cf.obligatorio, cf.vigente_desde
		 FROM project_compliance_frameworks pcf
		 JOIN compliance_frameworks cf ON cf.id = pcf.framework_id AND cf.deleted_at IS NULL
		 JOIN compliance_framework_controls fc ON fc.framework_id = cf.id
		 JOIN compliance_controls cc ON cc.id = fc.control_id AND cc.deleted_at IS NULL
		 WHERE pcf.project_id = $1 AND pcf.activo
		 ORDER BY cc.slug, cf.slug`, projectID)
	if err != nil {
		return nil, fmt.Errorf("controles exigidos: %w", err)
	}
	defer rows.Close()

	var out []ControlExigido
	for rows.Next() {
		var c ControlExigido
		var vigenteDesde *time.Time
		if err := rows.Scan(&c.ControlID, &c.ControlSlug, &c.Nombre, &c.FrameworkSlug,
			&c.Referencia, &c.Obligatorio, &vigenteDesde); err != nil {
			return nil, fmt.Errorf("scan control exigido: %w", err)
		}
		c.Vigente = Framework{VigenteDesde: vigenteDesde}.Vigente(ahora)
		out = append(out, c)
	}
	return out, rows.Err()
}

// RegistrarEstado deja el estado de un control para el proyecto. Idempotente por
// (project_id, control_id): una re-evaluación actualiza en vez de acumular filas.
func (s *Service) RegistrarEstado(ctx context.Context, projectID, controlID uuid.UUID,
	estado, evidencia string, actorID *uuid.UUID,
) error {
	if !estadosValidos[estado] {
		return ErrEstadoInvalido
	}
	_, err := s.q(ctx).Exec(ctx,
		`INSERT INTO project_control_status
		     (project_id, control_id, estado, evidencia, evaluado_por_id)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (project_id, control_id)
		 DO UPDATE SET estado = EXCLUDED.estado, evidencia = EXCLUDED.evidencia,
		               evaluado_por_id = EXCLUDED.evaluado_por_id, evaluado_at = NOW()`,
		projectID, controlID, estado, evidencia, actorID)
	if err != nil {
		return fmt.Errorf("registrar estado: %w", err)
	}
	return nil
}

// BuscarPorSlug resuelve un marco del catálogo. La edición vacía toma la única que haya; si hay
// varias ediciones cargadas, exige elegir en vez de devolver una arbitraria.
func (s *Service) BuscarPorSlug(ctx context.Context, slug, edicion string) (*Framework, error) {
	rows, err := s.q(ctx).Query(ctx,
		`SELECT `+columnasFramework+` FROM compliance_frameworks
		 WHERE slug = $1 AND ($2 = '' OR edicion = $2) AND deleted_at IS NULL`, slug, edicion)
	if err != nil {
		return nil, fmt.Errorf("buscar marco: %w", err)
	}
	marcos, err := escanearFrameworks(rows)
	if err != nil {
		return nil, err
	}
	if len(marcos) == 0 {
		return nil, ErrMarcoNoEncontrado
	}
	if len(marcos) > 1 {
		return nil, fmt.Errorf("%w: hay %d ediciones de %q, indicá cuál",
			errors.New("edición ambigua"), len(marcos), slug)
	}
	return &marcos[0], nil
}

// BuscarControlPorSlug resuelve un control del catálogo. Devuelve ErrControlNoEncontrado en vez de
// un uuid.Nil silencioso: escribir el estado de un control inexistente dejaría una fila que no
// aparece en ningún reporte.
func (s *Service) BuscarControlPorSlug(ctx context.Context, slug string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.q(ctx).QueryRow(ctx,
		`SELECT id FROM compliance_controls WHERE slug = $1 AND deleted_at IS NULL`, slug).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrControlNoEncontrado
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("buscar control: %w", err)
	}
	return id, nil
}

// GuardDeFuente rechaza ingestar el texto completo de un marco no redistribuible. Es el punto
// donde el campo fuente_tipo deja de ser metadata y se vuelve un guard.
func (s *Service) GuardDeFuente(f Framework) error {
	if !f.AdmiteTextoCompleto() {
		return fmt.Errorf("%w (%s)", ErrFuenteNoRedistribuible, f.Slug)
	}
	return nil
}
