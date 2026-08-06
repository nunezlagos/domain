-- migration: 000288_webhooks_project_scope_rls
-- author: nunezlagos
-- issue: DOMAINSERV-240
-- description: webhooks recupera un eje de scope (project_id) con RLS, recupera el índice único
--   de slug que la 000142 se llevó de arrastre, y se limpian los secretos que webhook_deliveries
--   persistía en claro
-- breaking: no — medido en prod: 0 filas en webhooks y 0 en webhook_deliveries, así que ningún
--   dato existente cambia de comportamiento
-- estimated_duration: <1s (tabla vacía; ENABLE/FORCE y CREATE POLICY son cambios de catálogo)
--
-- POR QUÉ, y son tres defectos distintos que comparten la misma causa raíz
--
-- 1. NO HAY EJE DE SCOPE. La 000017 creó webhooks con organization_id NOT NULL y un
--    UNIQUE (organization_id, slug). La 000142 dropeó organization_id de todas las tablas con un
--    bloque DO genérico, y Postgres se llevó el constraint dependiente junto con la columna.
--    Desde entonces webhooks no tiene NI organization_id NI project_id NI RLS, así que cualquier
--    query de management ve la instancia entera.
--
-- 2. EL UNIQUE DE SLUG DESAPARECIÓ CON ÉL. service.go mapea UniqueViolation a ErrSlugTaken
--    (:174-177), pero esa rama es INALCANZABLE hoy: dos webhooks pueden compartir slug y
--    GetWebhookBySlug es :one, así que /receive resuelve a una fila arbitraria de las dos.
--
-- 3. webhook_deliveries GUARDA EL SECRETO EN CLARO. collectHeaders copia r.Header entero, y para
--    gitlab el secreto ES el header: `token == string(secret)` (handler/webhook.go:96-97). O sea
--    que el log de entregas contiene la credencial que valida las entregas.
--
-- EL EJE ES project_id Y NO organization_id, y esto se midió antes de elegir: el eje org es
-- DECORATIVO en esta instancia porque internal/auth/apikey/store.go:30 define un
-- canonicalOrgID fijo y :113/:213 lo asignan a TODA credencial. Un RLS por org no aislaría
-- nada. project_id es el eje que la plataforma ya adoptó en 000287 (DOMAINSERV-187).
--
-- EL ORDEN DE LOS STATEMENTS NO ES ARBITRARIO, y dos de sus razones son sutiles:
--   * el scrub de headers va ANTES de habilitar RLS. Al revés, el UPDATE quedaría filtrado por
--     la policy y limpiaría CERO filas sin fallar — un scrub que no scrubbea.
--   * el UPDATE que apaga los webhooks huérfanos va DESPUÉS del ADD COLUMN. Antes la columna no
--     existe y el statement falla con 42703; el plan original tenía este orden invertido.
--
-- MEDIDO EN PROD ANTES DE APLICAR (2026-08-06): webhooks 0 filas y 0 slugs duplicados vivos;
-- webhook_deliveries 0 filas y 0 con X-Gitlab-Token; headers es jsonb. Los tres statements de
-- limpieza son no-ops acá, y existen para los ambientes que sí tengan datos.

-- 1. Duplicados de slug ANTES del índice único: se queda el más viejo. Sin esto el CREATE UNIQUE
--    INDEX falla en cualquier base que tenga dos webhooks con el mismo slug.
UPDATE webhooks SET deleted_at = now()
WHERE deleted_at IS NULL
  AND id NOT IN (
    SELECT DISTINCT ON (slug) id
    FROM webhooks
    WHERE deleted_at IS NULL
    ORDER BY slug, created_at
  );

-- 2. Scrub de los headers sensibles ya persistidos. VA ANTES del ENABLE ROW LEVEL SECURITY: con
--    la policy activa este UPDATE vería 0 filas y no fallaría, o sea que el secreto sobreviviría
--    en silencio. El operador `-` de jsonb quita la clave entera; se pierde la presencia del
--    header, y en las filas viejas eso es preferible a conservar el valor.
UPDATE webhook_deliveries
SET headers = headers - 'X-Gitlab-Token' - 'X-Hub-Signature' - 'X-Hub-Signature-256'
                      - 'X-Domain-Signature' - 'Authorization' - 'Proxy-Authorization'
                      - 'Cookie' - 'Set-Cookie'
WHERE headers IS NOT NULL;

-- 3. El eje de scope. ON DELETE CASCADE porque un webhook sin proyecto no tiene forma de
--    resolverse: es más honesto que quede huérfano-borrado que huérfano-inalcanzable.
ALTER TABLE webhooks ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id) ON DELETE CASCADE;

-- CONCURRENTLY NO se usa acá y no es un olvido: golang-migrate manda el archivo entero en un
-- solo Exec, que pgx ejecuta como implicit transaction block, y CREATE INDEX CONCURRENTLY dentro
-- de una transacción falla con 25001. Precedentes del mismo override en este repo: 000161 y
-- 000272:13. La tabla tiene 0 filas, así que el lock es instantáneo.
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX IF NOT EXISTS webhooks_project_id_idx ON webhooks (project_id);

-- El único parcial: un slug borrado puede reusarse, que es lo que hace viable el
-- delete+create como camino de rotación de secreto.
-- domain-lint-ignore-next: require-concurrent-index
CREATE UNIQUE INDEX IF NOT EXISTS webhooks_slug_global_uniq ON webhooks (slug) WHERE deleted_at IS NULL;

-- 4. Fail-closed de los huérfanos, DESPUÉS de que la columna exista. Una fila sin project_id es
--    invisible para el management bajo RLS; dejarla enabled la convertiría en un endpoint
--    fantasma que dispara flows y que nadie puede listar ni borrar.
UPDATE webhooks SET enabled = false, updated_at = now() WHERE project_id IS NULL;

-- 5. current_project_id() con CREATE OR REPLACE: la 000287 ya la crea, y repetirla acá hace que
--    esta migración no dependa del orden ni de que un down de 000287 la haya dejado sin función.
--    nullif es lo que importa: sin él, current_setting devuelve '' cuando el GUC no está seteado
--    y el ::uuid revienta con ERROR en vez de dar NULL — la query fallaría ruidosamente en lugar
--    de devolver cero filas, que es el contrato que el RLS necesita.
CREATE OR REPLACE FUNCTION current_project_id() RETURNS UUID AS $$
BEGIN
  RETURN nullif(current_setting('app.current_project_id', true), '')::uuid;
EXCEPTION WHEN OTHERS THEN
  RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

ALTER TABLE webhooks ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhooks FORCE ROW LEVEL SECURITY;
CREATE POLICY webhooks_project_isolation ON webhooks
  FOR ALL TO PUBLIC
  USING (project_id = current_project_id())
  WITH CHECK (project_id = current_project_id());

-- webhook_deliveries NO tiene project_id y no se le agrega: su proyecto es el del webhook padre,
-- y desnormalizarlo abriría la puerta a una delivery con un proyecto distinto al de su webhook.
-- Mismo criterio que knowledge_chunks en 000287.
ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_deliveries FORCE ROW LEVEL SECURITY;
CREATE POLICY webhook_deliveries_project_isolation ON webhook_deliveries
  FOR ALL TO PUBLIC
  USING (EXISTS (
    SELECT 1 FROM webhooks w
    WHERE w.id = webhook_deliveries.webhook_id
      AND w.project_id = current_project_id()
  ))
  WITH CHECK (EXISTS (
    SELECT 1 FROM webhooks w
    WHERE w.id = webhook_deliveries.webhook_id
      AND w.project_id = current_project_id()
  ));
