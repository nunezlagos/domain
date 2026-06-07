# Design: HU-03.3-prompts-storage

## Decisión arquitectónica

**Tabla prompts + buffer process-local con batch insert asincrónico.**

```
prompts
├── id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── session_id  TEXT REFERENCES sessions(id) ON DELETE SET NULL
├── content     TEXT NOT NULL
├── tsv         TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', content)) STORED
└── created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
```

**Buffer process-local:**
```
PromptBuffer
├── ch      chan Prompt           -- buffered, size = 100
├── batch   []Prompt              -- acumulador para batch insert
├── ticker  *time.Ticker          -- cada 500ms
├── done    chan struct{}         -- señal de shutdown
├── wg      sync.WaitGroup       -- esperar flush final
└── store   PromptStore           -- donde hacer flush
```

**Flujo:**
1. `SavePrompt()` → `select { case ch <- p: default: log.Warn("buffer full, dropping prompt") }`
2. Goroutine worker: `select { case p := <-ch: batch = append(batch, p); case <-ticker.C: flush(); case <-done: flush(); return }`
3. `flush()` → `store.BatchInsert(batch)` → si ok, batch = nil; si error, log.Error + retry en próximo ciclo

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| Insert síncrono siempre | Bloquea la respuesta del agente; engram usa buffer por una razón |
| Redis como buffer externo | Dependencia adicional innecesaria; buffer en memoria alcanza |
| Kafka / NATS | Overkill absoluto para este volumen |
| Batch insert cada N solamente | Sin ticker, prompts pueden quedarse en buffer indefinidamente |

## Diagrama

```
Agent call: domain_mem_save(capture_prompt=true)
  └── MemoryService.SavePrompt(content)
        └── PromptBuffer.ch <- Prompt{...}   (non-blocking)
              │
              ▼ (goroutine worker)
        ┌──────────┬──────────┐
        │ ticker   │  ch      │  ← new prompts
        │ 500ms    │  event   │
        └─────┬────┴────┬─────┘
              ▼         ▼
        ┌──────────────────────┐
        │      flush()         │
        │ batch insert a PG    │
        └──────────────────────┘
```

## TDD plan

1. **Red**: Test: SavePrompt → buscar → encontrar
2. **Green**: Implementar store.Insert + store.Search
3. **Red**: Test: buffer con múltiples prompts → flush automático
4. **Green**: Implementar PromptBuffer con worker
5. **Red**: Test: shutdown → flush final → todos los prompts persistidos
6. **Green**: Implementar graceful shutdown
7. **Sabotaje**: detener worker antes de flush → prompts en buffer se pierden (documentado)

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Pérdida de prompts en crash | Documentado como best-effort; prompts críticos usar insert síncrono directo |
| Buffer overflow con alta carga | Channel size configurable; drop con log warning |
| Dependency cycle sessions→prompts | FK nullable, ON DELETE SET NULL |
