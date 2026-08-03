-- migration: 000282_reconcile_platform_policies_post_228 (down)
-- author: nunezlagos
-- issue: issue-54.8 (secuela de DOMAINSERV-228)
-- description: vuelve a marcar los 4 slugs como is_user_modified para sacarlos del gobierno
--   del seeder. NO restaura el body anterior de cada fila: el reset del flag es reversible,
--   el re-seed que lo sigue no lo es. Si hace falta el texto viejo, sale del historial de git
--   del catálogo, no de esta migración.
-- breaking: no
-- duration: <1s
UPDATE platform_policies
SET is_user_modified = true
WHERE is_active
  AND slug IN (
    'context-preservation',
    'guards-deben-ejecutarse',
    'reportar-consumo-de-memoria',
    'sdd-auto-trigger'
  );
