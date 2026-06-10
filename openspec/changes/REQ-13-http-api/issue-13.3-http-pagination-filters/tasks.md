# Tasks: issue-13.3-http-pagination-filters

## Backend

- [ ] Implementar `QueryParser`: parsea query params a `ListParams` struct
- [ ] Implementar whitelist de campos sorteables y filtrables por entidad
- [ ] Validar que campos de sort/filter estén en whitelist (400 si no)
- [ ] Implementar `OffsetPaginator`: genera SQL con OFFSET + LIMIT + total count
- [ ] Implementar `CursorPaginator`: genera SQL keyset pagination con cursor base64
- [ ] Implementar encode/decode de cursor (base64 JSON)
- [ ] Implementar sort dinámico: parsear `?sort=-campo1,campo2` → ORDER BY
- [ ] Implementar filtros exactos: `?campo=valor` → WHERE campo = valor
- [ ] Implementar filtros range: `?campo[gte]=x&campo[lte]=y`
- [ ] Implementar full-text search: `?q=terminos` → tsquery con ranking y headlines
- [ ] Integrar pagination en handler factory (CRUDHandlers.List)
- [ ] Response envelope: `{data, pagination}` con offset/limit/cursor/has_more/total
- [ ] Limitar máximo de resultados (default 20, max 100)
- [ ] Manejar edge cases: cursor inválido (400), sort no permitido (400), offset negativo

## Frontend

- [ ] N/A (API pura)

## Tests

- [ ] Test unitario: query parser con todos los params
- [ ] Test unitario: offset pagination SQL generation
- [ ] Test unitario: cursor pagination SQL generation
- [ ] Test unitario: sort dinámico single y multi-campo
- [ ] Test unitario: filtros exactos y range
- [ ] Test unitario: FTS query generation con tsquery
- [ ] Test de integración: offset y cursor devuelven mismos datos
- [ ] Test de integración: sort descendente y ascendente
- [ ] Test de integración: FTS ranking
- [ ] Test de errores: sort inválido (400), cursor corrupto (400)
- [ ] Sabotaje: eliminar whitelist check → test con sort inválido detecta

## Cierre

- [ ] Verificación manual: curl con pagination, filters, sort, search
- [ ] Suite verde: `go test ./internal/api/...`
- [ ] Performance test: 10k rows con offset vs cursor
