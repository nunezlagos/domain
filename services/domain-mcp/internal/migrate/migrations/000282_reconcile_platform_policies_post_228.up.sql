-- migration: 000282_reconcile_platform_policies_post_228
-- author: nunezlagos
-- issue: issue-54.8 (secuela de DOMAINSERV-228)
-- nota: toma la 282 y no la 280 porque el CHANGELOG reserva la 000280 para DOMAINSERV-185
--   parte B (RLS de knowledge, bloqueada por dos decisiones abiertas) y la 000281 para
--   DOMAINSERV-218. golang-migrate no exige contigüidad, y pisar un número reservado por otro
--   ticket cuesta más que dejar dos huecos.
-- description: resetea is_user_modified en los 4 slugs que DOMAINSERV-228 midió
--   divergentes entre el catálogo del seeder y la fila viva. Con el flag en false y el
--   PlatformPoliciesSeeder en Version 29, el re-seed reaplica el catálogo ya reconciliado.
--   El bump acompaña a esta migración: sin él, seeds.go:144 skippea con
--   applied_version >= Version() y el reset no cambia nada en ningún ambiente.
--
--   Adjudicación por slug (análisis adversarial, 4 veredictos sin refutar):
--     context-preservation        el catálogo es superconjunto estricto. La fila describe un
--                                 mem_search que devuelve el session_summary completo, pero
--                                 search_snippet.go:9 lo trunca a 200 bytes: describe un
--                                 comportamiento que el código ya no implementa.
--     guards-deben-ejecutarse     MERGE ya aplicado al catálogo en este mismo change: el
--                                 Corolario 2 (~35 líneas) solo existía en la fila y se portó
--                                 ANTES de este reset. Sin ese orden, el re-seed lo borraría.
--     reportar-consumo-de-memoria diferencia cosmética (H1 + wrapping); md5 normalizado igual.
--     sdd-auto-trigger            indistintos salvo 7 pares de backticks. La causa era
--                                 estructural —un raw-string de Go no puede contener backticks—
--                                 y por eso 000270 declaró esa edición legítima y la excluyó.
--                                 El catálogo ya no usa raw-string, así que ahora converge.
--
--   NO toca los otros 4 slugs con is_user_modified=true: delegar-lecturas-multiples (al día,
--   el flag está de más) ni cross-project-context / orca-worktree-conventions /
--   test-failure-root-cause-analysis, que no están en el catálogo y perderían su contenido.
-- breaking: no
-- duration: <1s
UPDATE platform_policies
SET is_user_modified = false
WHERE is_active
  AND is_user_modified
  AND slug IN (
    'context-preservation',
    'guards-deben-ejecutarse',
    'reportar-consumo-de-memoria',
    'sdd-auto-trigger'
  );
