-- migration: 000292_compliance_waivers
-- author: nunezlagos
-- issue: issue-56.5-fase-sdd-compliance-con-bloqueo-y-waiver
-- description: waivers de compliance — la vía de excepción auditada para un BLOCKER de la fase
--   sdd-compliance
-- breaking: no — tabla nueva, ninguna existente se toca
-- estimated_duration: <1s (tabla vacía; ENABLE/FORCE y CREATE POLICY son cambios de catálogo)
--
-- POR QUÉ EXISTE UNA VÍA DE EXCEPCIÓN
--
-- Un gate sin válvula de escape se vuelve insatisfacible y empuja al bypass permanente. Este repo
-- ya documentó ese modo de falla tres veces (DOMAINSERV-111, 175 y 195): cuando el único camino
-- para avanzar es desactivar el guard, alguien lo desactiva, y entonces deja de proteger SIEMPRE.
-- Un waiver con fricción —razón escrita obligatoria y registro auditable— es más seguro que un
-- BLOCKER duro que termina apagado.
--
-- POR QUÉ EN LA BASE Y NO EN EL FILESYSTEM
--
-- El commit-gate usa un archivo local de un solo uso (~/.local/state/domain/gate-bypass-<sesión>) y
-- para ese caso está bien. Acá no: un waiver de compliance tiene que ser AUDITABLE POR OTRO — esa
-- es su razón de ser—, y un archivo en el home de quien lo otorgó no lo es. Además tiene que
-- sobrevivir a la sesión: el flow puede retomarse días después.
--
-- razon es NOT NULL y con CHECK de longitud: un waiver sin razón es un bypass con otro nombre. El
-- CHECK usa btrim para que espacios en blanco no cuenten como razón.
--
-- El eje de RLS es project_id, igual que la 000287, 000288 y 000291: la 000142 borró
-- organization_id de todas las tablas y el eje org es decorativo en esta instancia.

CREATE TABLE IF NOT EXISTS compliance_waivers (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  -- el control cuya obligación se exceptúa. Sin FK a compliance_controls a propósito: un waiver
  -- tiene que sobrevivir a que el catálogo se reorganice, porque es un registro de auditoría
  control_slug   VARCHAR(100) NOT NULL,
  framework_slug VARCHAR(100) NOT NULL,
  -- un waiver sin razón escrita es un bypass con otro nombre
  razon        TEXT NOT NULL CHECK (length(btrim(razon)) >= 10),
  otorgado_por_id UUID REFERENCES users(id) ON DELETE SET NULL,
  otorgado_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- vencimiento opcional: un waiver "hasta que se implemente" no debería ser eterno, pero
  -- forzar una fecha llevaría a poner una cualquiera
  vence_at     TIMESTAMPTZ,
  revocado_at  TIMESTAMPTZ,
  -- el flow que lo motivó, para reconstruir por qué se otorgó
  flow_run_id  UUID
);

-- un waiver vivo por (proyecto, control, marco): re-otorgar actualiza en vez de acumular, y el
-- parcial permite que un waiver revocado no bloquee otorgar uno nuevo
-- domain-lint-ignore-next: require-concurrent-index
CREATE UNIQUE INDEX IF NOT EXISTS compliance_waivers_vivo_uniq
  ON compliance_waivers (project_id, control_slug, framework_slug)
  WHERE revocado_at IS NULL;

-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX IF NOT EXISTS compliance_waivers_project_idx ON compliance_waivers (project_id);

CREATE OR REPLACE FUNCTION current_project_id() RETURNS UUID AS $$
BEGIN
  RETURN nullif(current_setting('app.current_project_id', true), '')::uuid;
EXCEPTION WHEN OTHERS THEN
  RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

ALTER TABLE compliance_waivers ENABLE ROW LEVEL SECURITY;
ALTER TABLE compliance_waivers FORCE ROW LEVEL SECURITY;
CREATE POLICY compliance_waivers_isolation ON compliance_waivers
  FOR ALL TO PUBLIC
  USING (project_id = current_project_id())
  WITH CHECK (project_id = current_project_id());
