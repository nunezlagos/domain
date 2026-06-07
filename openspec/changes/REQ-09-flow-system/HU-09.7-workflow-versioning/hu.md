# HU-09.7-workflow-versioning

**Origen:** `REQ-09-flow-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer modificando un flow en producción
**Quiero** que las runs en vuelo terminen con la versión que iniciaron, y las nuevas usen la última publicada
**Para** evitar romper runs largos al editar el flow

## Modelo

- Cada save crea `flow_versions` con `version_number` incremental
- Una `flow_versions` puede estar `draft`, `published`, `deprecated`
- Solo `published` se puede invocar
- `flow_runs.flow_version_id` apunta a la versión congelada al iniciar
- Engine SIEMPRE lee la versión del run, no la actual del flow

## Criterios de aceptación

### Escenario 1: Save crea nueva versión draft

```gherkin
Dado que existe flow `deploy-prod` v3 published
Cuando PATCH /flows/:id con cambios
Entonces se crea `flow_versions` v4 draft (la v3 queda intacta)
Y la v4 NO puede invocarse aún (`published_at IS NULL`)
```

### Escenario 2: Publish

```gherkin
Dado que v4 está draft
Cuando POST /flows/:id/versions/:n/publish
Entonces v4 pasa a published_at = NOW()
Y nuevas invocaciones POST /flows/:id/run usan v4
Y v3 queda accesible pero ya no es la default
```

### Escenario 3: Runs en vuelo terminan con su versión

```gherkin
Dado que hay un run iniciado con v3 (todavía running)
Y se publica v4 con cambios incompatibles
Cuando el run continúa
Entonces sigue ejecutando steps de v3 (NOT v4)
Y al terminar queda registrado con `flow_version_id = v3_id`
```

### Escenario 4: Invocar versión específica

```gherkin
Dado que quiero correr v3 explícitamente
Cuando POST /flows/:id/run con `{"version":3}`
Entonces se usa v3 aunque v4 sea la published default
```

### Escenario 5: Deprecate

```gherkin
Dado que v2 es muy vieja
Cuando POST /flows/:id/versions/2/deprecate
Entonces v2.status = "deprecated"
Y intento de invocar v2 → 410 "version deprecated"
Y runs en vuelo (improbable pero posible) terminan OK
```

### Escenario 6: Diff entre versiones

```gherkin
Dado que existe v3 y v4
Cuando GET /flows/:id/versions/diff?from=3&to=4
Entonces devuelve diff JSON (json-patch RFC 6902)
Y se identifican breaking changes (step removed, type changed)
```

## Análisis breve

- **Qué pide:** tabla flow_versions + immutable spec por versión + run pinning + publish/deprecate lifecycle + diff
- **Esfuerzo:** M
- **Riesgos:** confusión entre "current" y "published"; storage acumulando versiones; migration de runs en vuelo en breaking changes
