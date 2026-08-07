-- migration: 000291_compliance_marcos_normativos
-- author: nunezlagos
-- issue: issue-56.4-marcos-normativos-por-proyecto-con-crosswalk
-- description: catálogo de marcos normativos (leyes y normas técnicas) + crosswalk de controles,
--   más las dos tablas por proyecto que declaran a qué marcos está afecto y en qué estado está
--   cada control
-- breaking: no — cinco tablas nuevas, ninguna existente se toca
-- estimated_duration: <1s (tablas vacías; ENABLE/FORCE y CREATE POLICY son cambios de catálogo)
--
-- POR QUÉ ESTA MIGRACIÓN EXISTE
--
-- El modelo de skills es opt-OUT: SkillApplicableIDs resuelve
-- `(s.project_id IS NULL OR s.project_id = :project_id) AND NOT EXISTS (… is_enabled = FALSE)`,
-- así que toda skill global auto-aplica a TODOS los proyectos y project_skills solo sirve para
-- excluir. No hay forma de expresar "esta obligación aplica solo si el proyecto la declara", que es
-- exactamente lo que el compliance necesita: un proyecto sin relación con Chile no puede quedar
-- afecto a la Ley 21.719 porque alguien registró una skill global.
--
-- LAS DOS MITADES TIENEN REGLAS DE ACCESO DISTINTAS, Y ESO ES EL PUNTO DEL DISEÑO:
--
--   CATÁLOGO (compliance_frameworks, compliance_controls, compliance_framework_controls) — SIN RLS.
--   Qué ES la Ley 21.719 no depende del proyecto que la mire. Ponerlo bajo RLS haría que las
--   consultas devolvieran CERO FILAS SIN ERROR y el sistema parecería no tener marcos cargados —
--   el mismo modo de falla que la 000287 con knowledge_chunks y la 000288 con webhooks, donde el
--   síntoma fue indistinguible de "no hay datos" hasta que alguien corrió los tests de integración.
--
--   POR PROYECTO (project_compliance_frameworks, project_control_status) — CON RLS.
--   Ahí sí: a qué marcos está afecto un proyecto y qué controles cumple es información suya, y el
--   aislamiento importa.
--
-- EL EJE ES project_id Y NO organization_id por lo mismo que la 000287 y la 000288: la 000142 borró
-- organization_id de todas las tablas y la 000143 dropeó organizations. Además el eje org es
-- decorativo en esta instancia — internal/auth/apikey/store.go define un canonicalOrgID fijo que se
-- asigna a toda credencial, así que un RLS por org no aislaría nada.
--
-- fuente_tipo NO ES METADATA DESCRIPTIVA, ES UN GUARD DE LICENCIA. Las leyes chilenas y el GDPR son
-- texto público y redistribuible, así que su articulado se puede ingestar completo a knowledge_docs
-- y citar literal. Las normas ISO/IEC son de PAGO y su redistribución está PROHIBIDA: de esas solo
-- se puede guardar número de cláusula, interpretación propia y evidencia de cumplimiento. Sin este
-- campo en el esquema, el error de licencia es cuestión de tiempo — alguien va a intentar ingestar
-- la ISO igual que se ingestó la 21.719.

-- ---------------------------------------------------------------------------
-- CATÁLOGO — global a la instancia, sin RLS
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS compliance_frameworks (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug          VARCHAR(100) NOT NULL,
  nombre        VARCHAR(255) NOT NULL,
  -- una ley obliga y se cumple; una norma técnica es voluntaria y se certifica. Tratarlas igual es
  -- lo que hace que un sistema reporte "incumplimiento" de algo que nadie está obligado a cumplir
  tipo          TEXT NOT NULL CHECK (tipo IN ('ley', 'reglamento', 'norma_tecnica', 'estandar_industria')),
  -- NULLABLE a propósito: ISO 27001 no es de ningún país. Forzar un valor obligaría a inventar uno
  jurisdiccion  VARCHAR(8),
  obligatorio   BOOLEAN NOT NULL DEFAULT FALSE,
  certificable  BOOLEAN NOT NULL DEFAULT FALSE,
  -- ISO 27001:2022 reorganizó el Anexo A respecto de :2013, así que una referencia de cláusula sin
  -- edición es ambigua. Las leyes usan '' porque no tienen ediciones en ese sentido
  edicion       VARCHAR(32) NOT NULL DEFAULT '',
  -- la 21.719 rige recién desde 2026-12-01: "te aplica" y "te va a aplicar" no son lo mismo, y sin
  -- esta fecha un marco declarado a futuro se reportaría hoy como incumplido
  vigente_desde DATE,
  fuente_tipo   TEXT NOT NULL DEFAULT 'solo_referencia'
                CHECK (fuente_tipo IN ('texto_libre', 'solo_referencia')),
  descripcion   TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at    TIMESTAMPTZ
);

-- el UNIQUE lleva la edición: dos ediciones de la misma norma conviven como filas distintas y sus
-- cláusulas no se mezclan
-- domain-lint-ignore-next: require-concurrent-index
CREATE UNIQUE INDEX IF NOT EXISTS compliance_frameworks_slug_edicion_uniq
  ON compliance_frameworks (slug, edicion) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS compliance_controls (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug             VARCHAR(100) NOT NULL,
  nombre           VARCHAR(255) NOT NULL,
  descripcion      TEXT,
  -- qué evidencia hace verificable este control; sin esto un control es una intención, no un check
  como_se_verifica TEXT,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at       TIMESTAMPTZ
);

-- domain-lint-ignore-next: require-concurrent-index
CREATE UNIQUE INDEX IF NOT EXISTS compliance_controls_slug_uniq
  ON compliance_controls (slug) WHERE deleted_at IS NULL;

-- EL CROSSWALK. Es la razón de ser del modelo: "cifrado de datos en reposo" lo exigen a la vez la
-- Ley 21.719 (deber de seguridad), el GDPR (Art. 32), ISO 27001 (Anexo A) y SOC 2. Sin esta tabla
-- ese control se escribe, implementa y audita CUATRO veces; con ella se evalúa una vez y el
-- resultado se reporta contra cada marco con SU propia referencia de artículo o cláusula.
CREATE TABLE IF NOT EXISTS compliance_framework_controls (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  framework_id UUID NOT NULL REFERENCES compliance_frameworks(id) ON DELETE CASCADE,
  control_id   UUID NOT NULL REFERENCES compliance_controls(id) ON DELETE CASCADE,
  -- 'Art. 32' en GDPR, 'A.8.24' en ISO: el mismo control se cita distinto en cada marco
  referencia   VARCHAR(120) NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (framework_id, control_id)
);

-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX IF NOT EXISTS compliance_framework_controls_control_idx ON compliance_framework_controls (control_id);

-- ---------------------------------------------------------------------------
-- POR PROYECTO — con RLS por app.current_project_id
-- ---------------------------------------------------------------------------

-- EL OPT-IN. La ausencia de fila significa NO APLICA: es el default, y es lo contrario del modelo
-- de skills. Un proyecto recién creado no está afecto a nada hasta que alguien lo declare.
CREATE TABLE IF NOT EXISTS project_compliance_frameworks (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  framework_id UUID NOT NULL REFERENCES compliance_frameworks(id) ON DELETE CASCADE,
  activo       BOOLEAN NOT NULL DEFAULT TRUE,
  activado_por_id UUID REFERENCES users(id) ON DELETE SET NULL,
  activado_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (project_id, framework_id)
);

-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX IF NOT EXISTS project_compliance_frameworks_project_idx
  ON project_compliance_frameworks (project_id) WHERE activo;

CREATE TABLE IF NOT EXISTS project_control_status (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  control_id   UUID NOT NULL REFERENCES compliance_controls(id) ON DELETE CASCADE,
  -- no_verificable es un estado de primera clase y no un "falta": hay controles de gobernanza que
  -- el código no puede demostrar, y confundirlos con incumplimientos infla el reporte
  estado       TEXT NOT NULL CHECK (estado IN ('ok', 'parcial', 'falta', 'no_verificable')),
  evidencia    TEXT,
  evaluado_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  evaluado_por_id UUID REFERENCES users(id) ON DELETE SET NULL,
  UNIQUE (project_id, control_id)
);

-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX IF NOT EXISTS project_control_status_project_idx ON project_control_status (project_id);

-- current_project_id() con CREATE OR REPLACE por lo mismo que la 000288: que esta migración no
-- dependa del orden ni de que un down previo la haya dejado sin función. El nullif es lo que
-- importa — sin él current_setting devuelve '' cuando el GUC no está seteado y el ::uuid revienta
-- con ERROR en vez de dar NULL, así que la query fallaría ruidosamente en lugar de devolver cero
-- filas, que es el contrato que el RLS necesita.
CREATE OR REPLACE FUNCTION current_project_id() RETURNS UUID AS $$
BEGIN
  RETURN nullif(current_setting('app.current_project_id', true), '')::uuid;
EXCEPTION WHEN OTHERS THEN
  RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

ALTER TABLE project_compliance_frameworks ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_compliance_frameworks FORCE ROW LEVEL SECURITY;
CREATE POLICY project_compliance_frameworks_isolation ON project_compliance_frameworks
  FOR ALL TO PUBLIC
  USING (project_id = current_project_id())
  WITH CHECK (project_id = current_project_id());

ALTER TABLE project_control_status ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_control_status FORCE ROW LEVEL SECURITY;
CREATE POLICY project_control_status_isolation ON project_control_status
  FOR ALL TO PUBLIC
  USING (project_id = current_project_id())
  WITH CHECK (project_id = current_project_id());
