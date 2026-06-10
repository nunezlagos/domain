# Proposal: issue-09.7-chunk-codec

## Intención

Crear un chunk codec para el cloud sync que genere IDs determinísticos via SHA-256 truncado, canonicalice payloads entre proyectos, y soporte encode/decode JSON para transmisión y almacenamiento.

## Scope

**Incluye:**
- `ChunkID(payload)` → SHA-256 truncado a 8 hex chars
- `CanonicalizeForProject(chunk, targetProject)` → normaliza payload
- `Encode(chunk)` → JSON serialization
- `Decode(data)` → JSON deserialization
- Validación de payloads vacíos
- Campos volátiles removidos en canonicalización

**No incluye:**
- Chunk storage o indexing (issue-09.3 server API)
- Chunk diff o merge (futuro)
- Compression (futuro)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Hash | SHA-256 del JSON canónico del payload, truncado a 8 hex chars |
| Canonicalización | JSON marshal/unmarshal con field stripping |
| Encode/Decode | `encoding/json` standard library |
| Chunk struct | `Chunk { ID, Project, Payload, ContentHash, CreatedAt }` |
