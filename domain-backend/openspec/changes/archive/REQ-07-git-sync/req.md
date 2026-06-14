# REQ-07-git-sync: Sincronización cross-machine mediante git chunks comprimidos: manifest, chunks gzipped JSONL, SHA-256 content hash, import/status.

**Estado:** activo
**Creado:** 2026-06-07

## Descripción

Sincronización cross-machine mediante git chunks comprimidos: manifest, chunks gzipped JSONL, SHA-256 content hash, import/status.

## Criterios de éxito

-

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-07.1-chunk-export-manifest | proposed | Gzipped JSONL chunks, SHA-256 content-addressed, manifest.json, --project/--all |
| issue-07.2-chunk-import | proposed | Import from manifest, INSERT OR IGNORE, atomic per-chunk, sync_chunks tracking |
| issue-07.3-sync-status | proposed | Local vs remote counts, manifest health check, SHA-256 verification |
