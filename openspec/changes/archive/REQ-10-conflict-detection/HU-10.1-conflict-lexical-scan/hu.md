# HU-10.1-conflict-lexical-scan

**Origen:** `REQ-10-conflict-detection`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** escanear observaciones en busca de conflictos léxicos usando FTS5
**Para** detectar entradas que hablan del mismo tema pero están duplicadas

**Como** desarrollador
**Quiero** controlar el scan con flags --dry-run, --apply, --max-insert, --since
**Para** revisar candidatos antes de insertarlos y limitar el volumen

## Criterios de aceptación

```gherkin
Scenario: FindCandidates usa FTS5 para encontrar overlapping lexical content
  Given hay dos observaciones con contenido similar "el servidor no responde" y "servidor caído sin respuesta"
  When se ejecuta FindCandidates()
  Then deben aparecer como candidates con lexical overlap > threshold

Scenario: Candidate se escribe en memory_relations con relation="candidate"
  Given se ejecuta FindCandidates() y encuentra un match
  When el scan se ejecuta sin --dry-run
  Then se inserta un registro en memory_relations con source_id, target_id, relation="candidate"

Scenario: --dry-run no escribe en memory_relations
  Given se ejecuta FindCandidates() con --dry-run
  When se encuentra un candidate
  Then el candidate se muestra en output
  And no se inserta en memory_relations

Scenario: --apply inserta candidates en memory_relations
  Given se ejecuta FindCandidates() con --apply
  When se encuentra un candidate
  Then se inserta en memory_relations
  And el output confirma la inserción

Scenario: --max-insert limita el número de candidates insertados
  Given hay 1000 candidates potenciales
  When se ejecuta FindCandidates() con --max-insert=50
  Then solo se insertan 50 candidates en memory_relations

Scenario: --since limita el scan a observaciones recientes
  Given hay observaciones de ayer y de hace un mes
  When se ejecuta FindCandidates() con --since=24h
  Then solo se escanean observaciones de las últimas 24 horas

Scenario: Memory_relations table almacena candidates con metadata
  Given se inserta un candidate
  When se consulta memory_relations
  Then tiene: source_id, target_id, relation="candidate", judgment_status="pending", confidence=(score), evidence="lexical:FTS5"

Scenario: FTS5 query usa tokenización apropiada
  Given observaciones con contenido técnico
  When se ejecuta la query FTS5
  Then la tokenización maneja correctamente: camelCase, snake_case, números, símbolos

Scenario: Scan reporta estadísticas al finalizar
  Given se ejecuta FindCandidates()
  When termina el scan
  Then reporta: total_scanned, candidates_found, candidates_inserted, duration

Scenario: Misma observación no se candidatea a sí misma
  Given una observación con contenido único
  When se ejecuta FindCandidates()
  Then no debe aparecer como candidate de sí misma
```

## Análisis breve

- **Qué pide realmente:** Algoritmo FindCandidates que usa FTS5 para encontrar lexical overlap entre observaciones, escribe candidates en memory_relations, soporta flags de control
- **Módulos sospechados:** `internal/conflict/` — `lexical.go` con FindCandidates
- **Riesgos / dependencias:** FTS5 query puede ser costosa en DB grandes; --max-insert y --since mitigan
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:** —
- **Acción derivada:** —
