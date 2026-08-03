# Reconciliar las 4 platform policies que divergen entre catalogo y produccion

## Why

8 de 32 platform policies tienen `is_user_modified=true` en produccion, asi que el
`PlatformPoliciesSeeder` no las pisa. De esas 8, DOMAINSERV-228 midio que 4 DIVERGEN del
catalogo del fuente: un fix mergeado con test verde puede no estar gobernando en produccion,
y ningun guard lo ve porque `platform_policies_divergencia_integration_test.go:42` es
`if !userModified && body != p.BodyMD` — las filas marcadas quedan fuera del guard por
contrato explicito, y `TestPlatformPolicies_FilaMarcada_PuedeDivergerSinRomperElGuard`
consagra ese silencio.

El caso con dano activo es `context-preservation`: la fila viva le dice al agente que
`domain_mem_search` devuelve el session_summary completo, cuando `search_snippet.go:9`
lo trunca a 200 bytes. El agente retoma el hilo creyendo que leyo un resumen entero
habiendo leido el 4%, que es peor que saber que le falta contexto.

La adjudicacion de cual version gana en cada slug sale de un analisis adversarial de 9
agentes (4 comparadores + 4 refutadores + 1 sintetizador); los 4 veredictos sobrevivieron
la refutacion con confianza alta.

| slug | gana | razon |
|---|---|---|
| context-preservation | catalogo | superconjunto estricto; la BD describe un comportamiento que el codigo ya no implementa |
| guards-deben-ejecutarse | merge | ~35 lineas del Corolario 2 viven SOLO en la BD |
| reportar-consumo-de-memoria | catalogo | diff cosmetico (H1 + wrapping), md5 normalizado identico |
| sdd-auto-trigger | indistinto | identicos salvo 7 pares de backticks |

## Scope

Entra:

- `platform_policies_seeder.go`: portar el "Corolario 2" de `guards-deben-ejecutarse` desde
  la fila de prod al `BodyMD` del catalogo, actualizando el estado real de sus cinco casos y
  reemplazando el simbolo muerto `llm.DefaultDim` por `migrate.EmbeddingDim`.
- `platform_policies_seeder.go:386-431`: `BodyMD` de `sdd-auto-trigger` de raw-string a
  string concatenado con `\n`, incorporando los 7 pares de backticks.
- `platform_policies_seeder.go:17`: `Version()` 28 -> 29 con su nota en el changelog inline.
- Migracion `000282_reconcile_platform_policies_post_228`: reset de `is_user_modified`
  acotado a los 4 slugs adjudicados.
- `platform_policies_reconcile_integration_test.go:51-81`: invertir el assert que hoy exige
  que `sdd-auto-trigger` conserve el flag.

Queda fuera:

| item | razon |
|---|---|
| Las otras 4 filas con `is_user_modified=true` | `delegar-lecturas-multiples` esta al dia (flag de mas); `cross-project-context`, `orca-worktree-conventions` y `test-failure-root-cause-analysis` ni siquiera estan en el catalogo. |
| El contrato del guard de divergencia | Que `is_user_modified` sea un opt-out silencioso y permanente es el defecto de fondo, pero cambiarlo altera el contrato de un guard vigente. |
| El truncado de `mem_context` en `search_snippet.go:36-49` | El paso 2 de `context-preservation` arrastra el mismo defecto que el paso 3 tenia. Es un gap del ganador, no contenido que este change pierda. |
| Los 49 tests de `domain-admin` sin pipeline | Se DOCUMENTA como caso abierto en el Corolario 2; agregarles CI es otro change. |

## Approach

1. Adjudicar por slug segun el analisis adversarial ya cerrado.
2. Llevar el catalogo al estado ganador ANTES de tocar el flag: primero el fuente correcto,
   despues el permiso para que el seeder lo aplique.
3. Resetear `is_user_modified` por migracion versionada acotada por `WHERE slug IN (...)`,
   calcada de `000270`, nunca por `domain_platform_policy_edit` — `policy/sql/query.sql:27`
   fuerza el flag en TRUE en cada edicion, asi que corregir la fila con esa tool la dejaria
   igual de blindada y silenciaria el guard para siempre.
4. Bumpear `Version()` a 29 en el mismo change: sin bump, `seeds.go:144`
   (`alreadyApplied := err == nil && appliedVersion >= s.Version()`) saltea el seeder y nada
   converge en ningun ambiente.
5. Invertir el assert que consagra el estado viejo como cambio de contrato documentado, no
   relajandolo ni salteandolo.

## Risks

| riesgo | mitigacion |
|---|---|
| Pisar `guards-deben-ejecutarse` perderia el Corolario 2, que solo vive en la BD | Su veredicto es merge, no catalogo. El Corolario sube al fuente ANTES de que la migracion habilite al seeder a escribir esa fila. |
| El reset del flag se aplica y el bump se olvida: prod queda igual y sin aviso | Ambos van en el mismo change y en el mismo deploy. El orden queda escrito: migracion, seeder con Version 29, verificacion. |
| Un `WHERE` mal acotado reabriria filas cuya edicion es legitima | La migracion lista los 4 slugs explicitamente. El abuse-case del MUST-3 lo fija como criterio de review. |
| Reescribir `sdd-auto-trigger` sin los backticks lo haria divergir de nuevo tras el reset | El criterio de aceptacion es md5 identico entre catalogo y fila, no inspeccion visual. |
| El catalogo portado documenta como abiertos casos que ya se cerraron | Los cinco casos se verificaron uno por uno contra el repo antes de redactar. |

## Testing

Los cinco casos del Corolario 2 se verificaron contra el repo antes de redactar el texto que
sube al catalogo:

| caso | estado | evidencia |
|---|---|---|
| REINDEX de los ivfflat | cerrado | `services/install.sh:869` ejecuta `domain embed-reindex`, ya no es una linea de `log` |
| cron de `skill_metrics` | cerrado | `docker-compose.yml:122` trae `DOMAIN_SKILL_METRICS_ENABLED: "${DOMAIN_SKILL_METRICS_ENABLED:-true}"` |
| rollup semanal | cerrado | `skill_metrics/sql/query.sql:172-200`: 10 columnas en el target y 10 expresiones en el SELECT, con `NOW() AS updated_at` |
| guards de templates de `domain-admin` | **ABIERTO** | los 2 archivos existen; ningun workflow los corre. Medido: 49 tests `test_*.py` en `services/domain-admin` y cero menciones en los 5 workflows de `.github/workflows/` |
| `llm.DefaultDim` | cerrado | el simbolo ya no existe; hoy es `migrate.EmbeddingDim = 1024` en `internal/migrate/embedding.go:12`, con guard en `embedding_test.go:17` |

Verificacion del change: los 2 tests de integracion invertidos contra Postgres real, y la
comparacion md5 entre el `BodyMD` del catalogo y la fila de prod para los 4 slugs.
