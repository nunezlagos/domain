# Tasks: issue-09.7-chunk-codec

## Backend

- [ ] **B1: Definir Chunk struct y SyncPayload**
      - `internal/cloud/chunk/types.go`
      - Campos ID, Project, Payload, ContentHash, CreatedAt, SyncStatus

- [ ] **B2: Implementar ChunkID()**
      - `internal/cloud/chunk/codec.go`
      - SHA-256 del payload normalizado (JSON compacto)
      - Truncar a primeros 4 bytes → 8 hex chars
      - Error si payload vacío

- [ ] **B3: Implementar CanonicalizeForProject()**
      - `internal/cloud/chunk/canonical.go`
      - Unmarshal SyncPayload
      - Remover campos volátiles (created_at, updated_at, sync_status)
      - Setear target project
      - Re-marshal a JSON

- [ ] **B4: Implementar Encode()**
      - `internal/cloud/chunk/codec.go`
      - Validar payload no vacío
      - Generar ChunkID si ID vacío
      - JSON marshal

- [ ] **B5: Implementar Decode()**
      - `internal/cloud/chunk/codec.go`
      - JSON unmarshal
      - Validar ID y payload presentes
      - Retornar Chunk

## Tests

- [ ] **T1: ChunkID retorna 8 caracteres hex**
- [ ] **T2: Mismo payload produce mismo ChunkID (determinismo)**
- [ ] **T3: Payload diferente produce ChunkID diferente**
- [ ] **T4: ChunkID con payload vacío retorna error**
- [ ] **T5: CanonicalizeForProject cambia project field**
- [ ] **T6: CanonicalizeForProject remueve created_at/updated_at/sync_status**
- [ ] **T7: Encode produce JSON válido**
- [ ] **T8: Decode restaura Chunk correctamente**
- [ ] **T9: Encode/Decode roundtrip preserva datos**
- [ ] **T10: Decode con JSON inválido retorna error**
- [ ] **T11: Decode con chunk sin ID retorna error**
- [ ] **T12: Canonicalización produce mismo ChunkID para contenido equivalente**
- [ ] **T13: Sabotaje — no compactar JSON antes de hash → IDs diferentes por whitespace → test cae → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cloud/chunk/... -v`
- [ ] Commit: `feat: chunk codec with deterministic ChunkID and canonicalization`
