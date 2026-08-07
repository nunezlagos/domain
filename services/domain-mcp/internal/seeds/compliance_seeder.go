package seeds

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ComplianceCatalogSeeder siembra el catálogo de marcos normativos, sus controles y el crosswalk
// entre ambos (issue-56.4). La data vive en compliance_catalog.go; acá solo se persiste.
//
// El catálogo es GLOBAL a la instancia y no lleva RLS, así que el seeder no necesita GUC de
// proyecto. Las tablas por proyecto no se siembran: un proyecto arranca sin marcos declarados
// porque el opt-in es explícito — sembrar una declaración sería justo el opt-out que este diseño
// evita.
type ComplianceCatalogSeeder struct{}

func (s *ComplianceCatalogSeeder) Name() string { return "compliance_catalog" }

// Version 1 — la carga inicial son las tres leyes/reglamentos con texto público. Subir la versión
// al agregar marcos o cambiar referencias; el catálogo se cambia con seeder + bump y nunca con SQL
// suelto (policy data-migration-methodology).
func (s *ComplianceCatalogSeeder) Version() int { return 1 }

// Order 46: después de known_errors (45). No depende de ningún otro catálogo.
func (s *ComplianceCatalogSeeder) Order() int { return 46 }

func (s *ComplianceCatalogSeeder) IsDevOnly() bool { return false }

// Run es idempotente por slug. Un re-seed actualiza los campos del catálogo pero NO toca las
// declaraciones de los proyectos, que viven en otras tablas.
func (s *ComplianceCatalogSeeder) Run(ctx context.Context, tx pgx.Tx, _ Env) (Report, error) {
	var rep Report
	if err := sembrarMarcos(ctx, tx, &rep); err != nil {
		return rep, err
	}
	if err := sembrarControles(ctx, tx, &rep); err != nil {
		return rep, err
	}
	return rep, sembrarCrosswalk(ctx, tx, &rep)
}

func sembrarMarcos(ctx context.Context, tx pgx.Tx, rep *Report) error {
	for _, m := range ComplianceFrameworkSeeds() {
		// los strings vacíos van a NULL: jurisdiccion vacía significa "no territorial" y
		// vigente_desde vacío significa "ya rige", no "rige el día cero"
		tag, err := tx.Exec(ctx, `
			INSERT INTO compliance_frameworks
				(slug, nombre, tipo, jurisdiccion, obligatorio, certificable,
				 vigente_desde, fuente_tipo, descripcion)
			VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,'')::date,$8,$9)
			ON CONFLICT (slug, edicion) WHERE deleted_at IS NULL
			DO UPDATE SET nombre = EXCLUDED.nombre, tipo = EXCLUDED.tipo,
			              jurisdiccion = EXCLUDED.jurisdiccion,
			              obligatorio = EXCLUDED.obligatorio,
			              certificable = EXCLUDED.certificable,
			              vigente_desde = EXCLUDED.vigente_desde,
			              fuente_tipo = EXCLUDED.fuente_tipo,
			              descripcion = EXCLUDED.descripcion, updated_at = NOW()
		`, m.Slug, m.Nombre, m.Tipo, m.Jurisdiccion, m.Obligatorio, m.Certificable,
			m.VigenteDesde, m.FuenteTipo, m.Descripcion)
		if err != nil {
			return fmt.Errorf("seed marco %s: %w", m.Slug, err)
		}
		contar(rep, tag.RowsAffected())
	}
	return nil
}

func sembrarControles(ctx context.Context, tx pgx.Tx, rep *Report) error {
	for _, c := range ComplianceControlSeeds() {
		tag, err := tx.Exec(ctx, `
			INSERT INTO compliance_controls (slug, nombre, descripcion, como_se_verifica)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (slug) WHERE deleted_at IS NULL
			DO UPDATE SET nombre = EXCLUDED.nombre, descripcion = EXCLUDED.descripcion,
			              como_se_verifica = EXCLUDED.como_se_verifica, updated_at = NOW()
		`, c.Slug, c.Nombre, c.Descripcion, c.ComoSeVerifica)
		if err != nil {
			return fmt.Errorf("seed control %s: %w", c.Slug, err)
		}
		contar(rep, tag.RowsAffected())
	}
	return nil
}

// sembrarCrosswalk resuelve los slugs a ids con un solo statement por vínculo. El INSERT ... SELECT
// evita traer los ids al cliente para volver a mandarlos: sin esto serían dos queries por vínculo.
func sembrarCrosswalk(ctx context.Context, tx pgx.Tx, rep *Report) error {
	for _, x := range ComplianceCrosswalkSeeds() {
		tag, err := tx.Exec(ctx, `
			INSERT INTO framework_controls (framework_id, control_id, referencia)
			SELECT cf.id, cc.id, $3
			FROM compliance_frameworks cf, compliance_controls cc
			WHERE cf.slug = $1 AND cf.deleted_at IS NULL
			  AND cc.slug = $2 AND cc.deleted_at IS NULL
			ON CONFLICT (framework_id, control_id)
			DO UPDATE SET referencia = EXCLUDED.referencia
		`, x.FrameworkSlug, x.ControlSlug, x.Referencia)
		if err != nil {
			return fmt.Errorf("seed crosswalk %s/%s: %w", x.FrameworkSlug, x.ControlSlug, err)
		}
		// cero filas significa que el marco o el control no existen: el crosswalk quedaría
		// incompleto en silencio, que es peor que fallar
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("crosswalk %s/%s: no existe el marco o el control",
				x.FrameworkSlug, x.ControlSlug)
		}
		contar(rep, tag.RowsAffected())
	}
	return nil
}

func contar(rep *Report, filas int64) {
	if filas == 1 {
		rep.Created++
		return
	}
	rep.Skipped++
}
