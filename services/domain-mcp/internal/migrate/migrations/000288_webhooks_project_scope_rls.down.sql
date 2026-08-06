-- migration: 000288_webhooks_project_scope_rls (down)
--
-- Revierte el schema en el orden inverso al de la up. Es exactamente reversible en estructura, y
-- por eso la up no es breaking: las policies se dropean, el RLS se apaga, los índices y la
-- columna se van.
--
-- LO QUE NO REVIERTE, y no puede: los tres statements de datos de la up.
--   * el scrub de headers borró los valores de los headers sensibles. No hay de dónde
--     reconstruirlos, y eso es deliberado: eran secretos que no debían estar ahí.
--   * el soft-delete de duplicados de slug y el enabled=false de los huérfanos tampoco se
--     deshacen. Un down que los reactivara volvería a crear los endpoints fantasma que la up
--     cerró, o sea que "revertir" sería reintroducir el defecto.
-- En prod los tres fueron no-ops (0 filas medidas), así que acá no hay nada perdido; en otro
-- ambiente, la fuente de verdad previa es el pg_dump que install.sh toma antes de migrar.
--
-- current_project_id() NO se dropea: la crea la 000287 y es suya. Dropearla acá dejaría el RLS de
-- knowledge_docs y knowledge_chunks apuntando a una función inexistente, que es un modo de falla
-- mucho peor que una función de más.

DROP POLICY IF EXISTS webhook_deliveries_project_isolation ON webhook_deliveries;
ALTER TABLE webhook_deliveries NO FORCE ROW LEVEL SECURITY;
ALTER TABLE webhook_deliveries DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS webhooks_project_isolation ON webhooks;
ALTER TABLE webhooks NO FORCE ROW LEVEL SECURITY;
ALTER TABLE webhooks DISABLE ROW LEVEL SECURITY;

DROP INDEX IF EXISTS webhooks_slug_global_uniq;
DROP INDEX IF EXISTS webhooks_project_id_idx;

ALTER TABLE webhooks DROP COLUMN IF EXISTS project_id;
