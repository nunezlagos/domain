# issue-09.7-chunk-codec

**Origen:** `REQ-09-cloud-sync`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** desarrollador de memoria
**Quiero** un chunk codec que genere IDs determinísticos y canónicos desde los payloads
**Para** que el cloud sync pueda identificar chunks únicos y evitar duplicados en la nube

**Como** desarrollador
**Quiero** canonicalización de chunks que normalice payloads al contexto del proyecto destino
**Para** que dos observaciones equivalentes en distintos proyectos compartan el mismo chunk ID

## Criterios de aceptación

```gherkin
Scenario: ChunkID genera hash determinístico
  Given un payload de observación
  When se genera ChunkID(payload)
  Then retorna string de 8 caracteres hexadecimales
  And el mismo payload siempre produce el mismo ChunkID

Scenario: ChunkID usa SHA-256 truncado
  Given un payload
  When se genera ChunkID
  Then internamente calcula SHA-256 del payload serializado
  And retorna solo los primeros 8 caracteres hex del hash

Scenario: ChunkID es único para distintos payloads
  Given dos payloads diferentes
  When se generan sus ChunkIDs
  Then los IDs son diferentes (colisiones extremadamente improbables)

Scenario: CanonicalizeForProject normaliza payload
  Given un chunk payload de proyecto A
  When se canonicaliza para proyecto B
  Then project reference se actualiza a B
  And el contenido semántico se preserva
  And metadata de sync se resetea (timestamps, sync_status)

Scenario: CanonicalizeForProject remueve campos volátiles
  Given un payload con created_at, updated_at, sync_status
  When se canonicaliza
  Then esos campos se remueven o resetean
  And solo contenido significativo permanece

Scenario: Encode genera JSON válido
  Given un Chunk struct con datos
  When se encodea a JSON
  Then output es JSON válido
  And incluye: chunk_id, project, payload, content_hash, created_at

Scenario: Decode restaura Chunk desde JSON
  Given un JSON codificado
  When se decodea
  Then retorna Chunk struct con todos los campos
  And ChunkID coincide con el hash del payload

Scenario: Encode/Decode roundtrip preserva datos
  Given un Chunk original
  When se encodea y luego se decodea
  Then el resultado es igual al original
  And ChunkID es consistente

Scenario: Payload nil o vacío produce error
  Given un payload nil o string vacío
  When se intenta generar ChunkID o encode
  Then retorna error "payload cannot be empty"

Scenario: Chunk con payload inválido (no JSON) falla en decode
  Given un string JSON inválido
  When se intenta decode
  Then retorna error de parseo JSON
```

## Análisis breve

- **Qué pide realmente:** Chunk codec package con ChunkID (SHA-256 truncado), CanonicalizeForProject, JSON encode/decode, validación de payloads
- **Módulos sospechados:** `internal/cloud/chunk/codec.go` — nuevo package
- **Riesgos / dependencias:** Colisiones de hash truncado (8 hex chars = 4 bytes = 4B posibilidades; suficiente para chunks por proyecto); canonicalización debe ser consistente
- **Esfuerzo tentativo:** S

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
