# Design: issue-09.7-chunk-codec

## Decisión arquitectónica

### Chunk struct

```go
// internal/cloud/chunk/types.go
package chunk

import "time"

type Chunk struct {
    ID           string          `json:"chunk_id"`
    Project      string          `json:"project"`
    Payload      json.RawMessage `json:"payload"`
    ContentHash  string          `json:"content_hash"`
    CreatedAt    time.Time       `json:"created_at"`
    SyncStatus   string          `json:"sync_status,omitempty"`
}

type SyncPayload struct {
    Title       string `json:"title,omitempty"`
    Content     string `json:"content,omitempty"`
    Tags        string `json:"tags,omitempty"`
    Project     string `json:"project,omitempty"`
    CreatedAt   string `json:"created_at,omitempty"`
    UpdatedAt   string `json:"updated_at,omitempty"`
    SyncStatus  string `json:"sync_status,omitempty"`
}
```

### ChunkID generation

```go
// internal/cloud/chunk/codec.go
func ChunkID(payload json.RawMessage) (string, error) {
    if len(payload) == 0 {
        return "", errors.New("payload cannot be empty")
    }
    // Normalize payload to canonical JSON (deterministic key order)
    var canonical bytes.Buffer
    if err := json.Compact(&canonical, payload); err != nil {
        return "", fmt.Errorf("invalid payload JSON: %w", err)
    }

    hash := sha256.Sum256(canonical.Bytes())
    return hex.EncodeToString(hash[:4]), nil // first 4 bytes = 8 hex chars
}
```

### CanonicalizeForProject

```go
// internal/cloud/chunk/canonical.go
func CanonicalizeForProject(payload json.RawMessage, targetProject string) (json.RawMessage, error) {
    var sp SyncPayload
    if err := json.Unmarshal(payload, &sp); err != nil {
        return nil, fmt.Errorf("cannot unmarshal payload: %w", err)
    }

    // Strip volatile fields
    sp.CreatedAt = ""
    sp.UpdatedAt = ""
    sp.SyncStatus = ""

    // Set target project
    sp.Project = targetProject

    result, err := json.Marshal(sp)
    if err != nil {
        return nil, fmt.Errorf("cannot marshal canonical payload: %w", err)
    }
    return result, nil
}
```

### Encode / Decode

```go
// internal/cloud/chunk/codec.go
func Encode(chunk Chunk) ([]byte, error) {
    if len(chunk.Payload) == 0 {
        return nil, errors.New("payload cannot be empty")
    }

    // Regenerate ID if empty
    if chunk.ID == "" {
        id, err := ChunkID(chunk.Payload)
        if err != nil {
            return nil, err
        }
        chunk.ID = id
    }

    return json.Marshal(chunk)
}

func Decode(data []byte) (Chunk, error) {
    var chunk Chunk
    if err := json.Unmarshal(data, &chunk); err != nil {
        return Chunk{}, fmt.Errorf("invalid chunk JSON: %w", err)
    }

    if chunk.ID == "" {
        return Chunk{}, errors.New("chunk missing ID")
    }
    if len(chunk.Payload) == 0 {
        return Chunk{}, errors.New("chunk missing payload")
    }

    return chunk, nil
}
```

### Roundtrip guarantee

```go
// ChunkID consistency after encode/decode
func TestRoundtrip(t *testing.T) {
    original := Chunk{
        Project: "test",
        Payload: json.RawMessage(`{"title":"hello"}`),
    }
    original.ID, _ = ChunkID(original.Payload)

    encoded, _ := Encode(original)
    decoded, _ := Decode(encoded)

    assert.Equal(t, original.ID, decoded.ID)
    assert.Equal(t, original.Project, decoded.Project)
    assert.JSONEq(t, string(original.Payload), string(decoded.Payload))
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| MD5 en vez de SHA-256 | MD5 es más rápido pero criptográficamente roto; SHA-256 es standard |
| Hash completo (64 chars) | 8 hex chars es suficiente para identificar chunks por proyecto; ahorra espacio en URLs/DB |
| Base64 en vez de hex | Hex es más legible para debugging; mismo tamaño informativo |
| UUID v4 para chunk ID | No determinístico; no permite deduplicación por contenido |

## TDD plan

1. **Red:** ChunkID retorna 8-char hex string → falla
2. **Green:** Implement SHA-256 truncado → pasa
3. **Red:** Mismo payload → mismo ChunkID → falla
4. **Green:** Deterministic hash → pasa
5. **Red:** CanonicalizeForProject cambia project → falla
6. **Green:** Implement canonicalización → pasa
7. **Red:** Encode/Decode roundtrip preserva datos → falla
8. **Green:** Implement codec → pasa
9. **Sabotaje:** ChunkID con payload vacío no valida → panic → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Colisión de hash en 8 hex chars | Probabilidad: 1/4B por par; aceptable para chunks por proyecto |
| JSON key ordering inconsistency | Usar json.Compact + json.Marshal para garantizar orden determinístico |
| Payload muy grande (>1MB) | SHA-256 maneja streams; límite en capa de aplicación |
