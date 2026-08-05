-- migration: 000287_knowledge_project_rls (down)
--
-- Revierte la segunda capa y deja knowledge_docs/knowledge_chunks como estaban: sin RLS, con
-- el WHERE de la query como única defensa. Es exactamente reversible en schema, y por eso la
-- up no es breaking — pero correr este down BAJA una defensa de aislamiento, no arregla nada.
-- Si el motivo para correrlo es que algo dejó de funcionar, el problema casi seguro es un
-- camino que no setea app.current_project_id: la lista de los verificados está en el .up.sql,
-- y agregar el GUC que falta es preferible a apagar el RLS de todo el knowledge.
--
-- El DROP POLICY va con IF EXISTS y el orden es el inverso al de la up: primero las policies,
-- después el FORCE/ENABLE, al final la función — que se dropea SIN CASCADE a propósito. Hoy
-- es exclusiva de estas dos tablas; si mañana otra policy la usa, este DROP falla
-- ruidosamente, y ese fallo es preferible a romperle el RLS a otra tabla en silencio.

DROP POLICY IF EXISTS knowledge_chunks_project_isolation ON knowledge_chunks;
ALTER TABLE knowledge_chunks NO FORCE ROW LEVEL SECURITY;
ALTER TABLE knowledge_chunks DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS knowledge_docs_project_isolation ON knowledge_docs;
ALTER TABLE knowledge_docs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE knowledge_docs DISABLE ROW LEVEL SECURITY;

DROP FUNCTION IF EXISTS current_project_id();
