# Tasks — reconciliar las 4 platform policies divergentes

> **Nota de orden**: la regla general del pipeline es "schema antes que code". Acá se invierte
> a proposito. El ADR 2 fija que el catalogo debe estar correcto ANTES de que la migracion
> habilite al seeder a escribir la fila: si el reset del flag corriera primero, el re-seed
> borraria el Corolario 2 de `guards-deben-ejecutarse`, que hoy solo existe en produccion.
> El orden aca es correctitud, no convencion.

## Code

- [ ] **t1** (grupo 1, 2h) — Portar el "Corolario 2" al `BodyMD` de `guards-deben-ejecutarse`
  en `platform_policies_seeder.go:523-537`, sin el H1 (que es el campo `Name`), conservando
  el encabezado fechado "cinco casos medidos el 2026-07-29", anotando cerrados los casos 1-3
  y ABIERTO el caso 4, y reemplazando `llm.DefaultDim` por `migrate.EmbeddingDim`.
  **Done cuando**: pasa `TestPlatformPolicies_Catalogo_GuardsDebenEjecutarse_ContieneElCorolario2`.

- [ ] **t2** (grupo 2, 1h) — Convertir el `BodyMD` de `sdd-auto-trigger`
  (`platform_policies_seeder.go:390-431`) de raw-string a string concatenado con `\n`,
  incorporando los 7 pares de backticks. Mismo patron que `context-preservation` en `:471`.
  **Done cuando**: pasa `TestPlatformPolicies_Catalogo_SddAutoTrigger_CoincideConLaFilaDeProd`
  (criterio md5, no inspeccion visual).
  **Grupo propio**: toca el mismo archivo que t1.

- [ ] **t3** (grupo 3, 1h) — Subir `Version()` de 28 a 29 en `platform_policies_seeder.go:17`
  y agregar la nota al changelog inline explicando que el bump habilita la reconciliacion
  de los 4 slugs.
  **Done cuando**: pasa `TestPlatformPoliciesSeeder_Version_EsMayorQueLaAplicadaEnProd`.
  **Grupo propio**: toca el mismo archivo que t1 y t2.

## Schema

- [ ] **t4** (grupo 4, 1h) — Crear `000282_reconcile_platform_policies_post_228.up.sql` y su
  `.down.sql`, calcadas de `000270`: `UPDATE platform_policies SET is_user_modified = FALSE
  WHERE is_active AND slug IN (los 4 slugs)`, con header obligatorio (migration, author,
  issue, description, breaking, duration) y comentario que explique la adjudicacion de cada
  slug.
  **Done cuando**: pasa `TestMigracion000282_ReseteaSoloLosCuatroSlugsAdjudicados`.
  **Depende de**: t1, t2, t3 — el catalogo tiene que estar ganador antes del reset.

## Tests

- [ ] **t5** (grupo 5, 2h) — Escribir los 3 guards nuevos:
  `TestPlatformPolicies_Catalogo_GuardsDebenEjecutarse_ContieneElCorolario2`,
  `TestPlatformPolicies_Catalogo_SddAutoTrigger_CoincideConLaFilaDeProd` y
  `TestMigracion000282_ReseteaSoloLosCuatroSlugsAdjudicados`. Los dos ultimos con build tag
  `//go:build integration` contra Postgres real: la diferencia entre fuente y fila la sabe la
  base, no Go.
  **Done cuando**: los 3 corren verdes y cada uno falla con su sabotaje del design.

- [ ] **t6** (grupo 5, 1h) — Invertir el assert de
  `platform_policies_reconcile_integration_test.go:51-81`, que hoy exige que
  `sdd-auto-trigger` CONSERVE `is_user_modified=true`, dejando registrada en el comentario la
  causa raiz: es cambio de contrato legitimo (categoria c de
  `test-failure-root-cause-analysis`), no un test roto que se relaja.
  **Done cuando**: el test verde contra la migracion nueva y el comentario cita issue-54.8.
  **Archivo distinto de t5**: puede ir en paralelo.

## Sabotage

- [ ] **t7** (grupo 6, 1h) — Ejecutar los 5 sabotajes del design uno por uno, verificando que
  cada test falla por la razon correcta, y restaurar. El critico es el de t4: quitar el
  `WHERE slug IN (...)` debe hacer que el test cuente 8 filas reseteadas en vez de 4 — es el
  abuse-case del MUST-3 hecho ejecutable.
  **Done cuando**: los 5 sabotajes dieron rojo por la razon esperada y el arbol quedo limpio
  (verificado por sha256, no por `diff`).

## Docs

- [ ] **t8** (grupo 6, 1h) — Agregar la entrada en `CHANGELOG.md` bajo Unreleased, y anotar
  en el ticket DOMAINSERV-228 que su causa raiz (el flag como opt-out silencioso) queda
  documentada pero NO resuelta por este change.
  **Archivo distinto de t7**: puede ir en paralelo.

## Verify

- [ ] **t9** (grupo 7, 1h) — Auditar el change completo: (1) ninguna funcion nueva supera 50
  lineas —`PlatformPolicyCatalog` ya lleva su `size-lint:allow` por ser catalogo de datos—,
  (2) inputs validados en boundaries (no aplica: no hay input de usuario nuevo), (3) sin
  secretos hardcodeados, (4) sin N+1 nuevas, (5) la suite pasa localmente, (6) sin codigo
  muerto, (7) el md5 del catalogo coincide con el de las 4 filas de prod tras el re-seed.
  **Va sola en el ultimo grupo**: depende de todo lo anterior.

## Orden de ejecucion

```
grupo 1 (t1) → grupo 2 (t2) → grupo 3 (t3) → grupo 4 (t4)
  → grupo 5 (t5 ∥ t6) → grupo 6 (t7 ∥ t8) → grupo 7 (t9)
```

t1, t2 y t3 van secuenciales pese a ser conceptualmente independientes: las tres tocan
`platform_policies_seeder.go` y correrlas en paralelo genera conflicto de edicion.
