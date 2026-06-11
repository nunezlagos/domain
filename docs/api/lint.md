# API Response-Shape Linter

> issue-13.9 — `cmd/response-shape-lint`
> Convenciones normativas: `.claude/rules/api.md`

Garantiza que todos los handlers HTTP respeten el envelope canónico
(`{data}` / `{error}`) y que el surface del API no cambie sin revisión.

## Cómo correr

```bash
make api-lint             # verifica handlers + rutas + snapshots
make api-snapshot-update  # regenera snapshots tras un cambio intencional
```

CI: job `response-shape-lint` en `ci.yml` bloquea el merge.

## Qué valida

1. **Shape de respuestas** (AST sobre `internal/api/handler/`):
   los handlers `func (a *API) x(w, r)` solo pueden responder vía
   `writeData`/`writeError`. Prohibido: `w.Write`, `w.WriteHeader`
   (salvo 204/304), `json.NewEncoder(w)`, `fmt.Fprintf(w, ...)`,
   `io.WriteString/Copy(w, ...)`.
2. **Rutas** (regex sobre `api.go`):
   - segmentos kebab-case (nada de `snake_case` ni mayúsculas)
   - handlers `create*` de rutas POST deben responder `http.StatusCreated`
3. **Snapshots** (`internal/api/handler/testdata/api/`):
   - `endpoint_shapes.json` — método + path + handler de cada ruta
   - `error_codes.json` — códigos machine-readable usados en `writeError`

   Cualquier drift (endpoint nuevo/cambiado, código de error nuevo) falla el
   lint hasta que se corra `-update` — el diff del snapshot queda visible en
   el PR para review explícito del cambio de contrato.

## Workflow al cambiar el API

1. Agregás/cambiás un endpoint o error code
2. `make api-lint` → falla con "snapshot drift"
3. `make api-snapshot-update` → regenera; el diff JSON entra al PR
4. Review humano del diff = aprobación del cambio de contrato

## Tests

`cmd/response-shape-lint/`: 15 tests — shape (8 fixtures testdata), rutas
snake_case, POST create sin 201, drift sin update, update regenera,
snapshot faltante, y `TestRealAPI_SnapshotsUpToDate` que verifica los
snapshots reales del repo en cada `go test`.
