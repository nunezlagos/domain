# Diseño — reconciliar las 4 platform policies divergentes

## Decisions

### ADR 1 — El reset de `is_user_modified` va por migracion versionada

Para devolver una fila de `platform_policies` al gobierno del seeder se usa una migracion
golang-migrate con `UPDATE` acotado por `WHERE slug IN (...)`, calcada de
`000270_reconcile_stale_user_modified_policies`.

`domain_platform_policy_edit` queda descartada: `policy/sql/query.sql:27`
(`UpdatePlatformPolicy`) fuerza `is_user_modified = TRUE` en cada edicion. Corregir el body
con esa tool deja la fila IGUAL de blindada, y el guard de divergencia la sigue tolerando
para siempre — que es exactamente el modo de falla que este change cierra.

La policy `data-migration-methodology` lo prescribe explicitamente en su seccion
`is_user_modified`: "Reconciliar una fila stale mal-congelada = migracion quirurgica que
resetea el flag + re-seed".

Patron: forward-only data migration.

### ADR 2 — `guards-deben-ejecutarse` se mergea hacia el catalogo; no se pisa la fila

El "Corolario 2" (~35 lineas) que hoy solo existe en produccion sube al `BodyMD` ANTES de
que la migracion habilite al seeder a escribir esa fila.

**El orden es correctitud, no estilo**: si la migracion corriera primero, el re-seed borraria
el texto de la fila. Es el unico de los 4 slugs donde la BD tiene contenido exclusivo.

Al portar se conserva el encabezado fechado ("cinco casos medidos el 2026-07-29") y se anota
el estado ACTUAL de cada caso: la fecha es evidencia, el estado es lo que envejece.

Patron: merge-before-reset.

### ADR 3 — `sdd-auto-trigger` pasa de raw-string a string concatenado

La unica diferencia con la fila son 7 pares de backticks, y la causa es **estructural**: un
raw-string de Go se delimita con backtick, asi que no puede contenerlos. Quien escribio la
entrada no pudo ponerlos; quien edito la fila si.

Eso explica por que `000270:7-8` declaro esa edicion "legitima" y la excluyo del reset — fue
la decision correcta con la informacion de entonces. Cambiar el tipo de literal remueve la
causa, y recien por eso ahora se puede incluir en el reset sin que nadie pierda nada.

Patron: eliminar la restriccion del medio en vez de convivir con su sintoma.

## Alternatives

| decision | alternativa rechazada | por que |
|---|---|---|
| reset por migracion | `domain_platform_policy_edit` | re-arma el flag en TRUE: la fila queda igual de blindada y el guard mudo para siempre |
| reset por migracion | `UPDATE` suelto contra prod | no versionado, no revisable, no reproducible; viola `data-migration-methodology` |
| merge del Corolario 2 | pisar la fila con el catalogo | perderia 35 lineas que no existen en ningun otro lado |
| merge del Corolario 2 | dejar la fila marcada | perpetua el opt-out: el proximo fix tampoco llegaria |
| string concatenado | dejar el raw-string | el slug queda fuera del gobierno del seeder para siempre |
| string concatenado | pisar la fila sin backticks | degrada markdown que el operador agrego deliberadamente |

## Data Flow

```
catalogo (platform_policies_seeder.go)
   │  1. merge del Corolario 2 + backticks + Version() 29
   ▼
migracion 000282
   │  2. is_user_modified = FALSE en los 4 slugs
   ▼
seeds.go RunAll
   │  3. appliedVersion(28) < Version()(29) → el seeder YA NO skippea
   ▼
UPSERT con WHERE is_user_modified = FALSE
   │  4. las 4 filas ahora SI se pisan con el catalogo ganador
   ▼
platform_policies (prod converge con el fuente)
```

El paso 3 es el que se olvida: sin bump, `seeds.go:144`
(`alreadyApplied := err == nil && appliedVersion >= s.Version()`) saltea el seeder aunque el
flag este limpio, y nada converge en ningun ambiente.

## TDD Plan

| # | test | que prueba | sabotaje |
|---|---|---|---|
| 1 | `TestPlatformPolicies_Catalogo_GuardsDebenEjecutarse_ContieneElCorolario2` | que el `BodyMD` del catalogo incluya la seccion portada y el simbolo `migrate.EmbeddingDim` | borrar la linea `"## Corolario 2: una feature que nunca se ejecuto tampoco esta cubierta\n\n" +` del catalogo → el test debe nombrar ese slug y solo ese |
| 2 | `TestPlatformPolicies_Catalogo_SddAutoTrigger_CoincideConLaFilaDeProd` | md5 del `BodyMD` == md5 del `body_md` de la fila | sacar un par de backticks de `domain_orchestrate` en el catalogo → md5 distinto, el test falla |
| 3 | `TestMigracion000282_ReseteaSoloLosCuatroSlugsAdjudicados` | tras la migracion, exactamente 4 filas con `is_user_modified=false` y las otras 4 marcadas intactas | quitar el `WHERE slug IN (...)` → resetea las 8, el test cuenta 8 y falla |
| 4 | `TestPlatformPoliciesSeeder_Version_EsMayorQueLaAplicadaEnProd` | `Version() == 29` | dejar `Version()` en 28 → `alreadyApplied` es true y el seeder skippea; el test lo atrapa antes del deploy |
| 5 | `TestMigracion000282_ElHeaderAdvierteLaDependenciaDelBinario` | que el header exija el binario con `Version() >= 29`, porque aplicar la migracion con el catalogo viejo borraria el Corolario 2 de prod | cambiar el texto → `does not contain "Version() >= 29"` |

Los tests 2, 3 y 5 son de INTEGRACION contra Postgres real: la diferencia entre lo que dice
el fuente y lo que dice la fila la sabe la base, no el codigo Go. Ese es precisamente el
punto de DOMAINSERV-228 — todos los guards previos leian el fuente como texto y por eso
estaban verdes con prod vieja.

## Risk Mitigation

| riesgo | mitigacion |
|---|---|
| pisar `guards-deben-ejecutarse` perderia el Corolario 2 | merge, no catalogo; y el orden fuente→flag lo garantiza |
| el flag se limpia y el bump se olvida | ambos en el mismo change y deploy; test 4 lo fija |
| un `WHERE` ancho reabriria filas legitimas | los 4 slugs listados explicitamente; test 3 con sabotaje |
| `sdd-auto-trigger` vuelve a divergir tras el reset | criterio md5, no inspeccion visual (test 2) |
| el Corolario documenta como abiertos casos ya cerrados | los 5 verificados uno por uno contra el repo antes de redactar |

### Nota de seguridad (causal_disposition = introduced)

Este change **resetea deliberadamente** un flag cuyo proposito es proteger customizaciones
del operador. El riesgo atribuible a este cambio es que un `WHERE` mal acotado reabra filas
cuya edicion SI era legitima. La mitigacion esta horneada en el diseno —los 4 slugs van
listados, y el test 3 falla si alguien amplia el alcance— no delegada a una task aparte.

Nota informativa (pre-existente, NO se aborda aca): ningun `UPDATE` del codigo pone
`is_user_modified = FALSE` en `platform_policies`. El unico camino de vuelta al gobierno del
seeder es una migracion. Es una observacion sobre el diseno existente.
