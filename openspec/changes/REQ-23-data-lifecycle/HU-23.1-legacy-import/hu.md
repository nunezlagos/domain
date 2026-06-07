# HU-23.1-legacy-import

**Origen:** `REQ-23-data-lifecycle`
**Persona:** org-admin, org-member
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario migrando desde otra herramienta
**Quiero** importar markdown vault / JSON dump / export Notion / Obsidian vault a Domain
**Para** consolidar conocimiento sin reescribirlo

## Criterios de aceptación

### Escenario 1: Importer markdown vault

```gherkin
Dado que cargo ZIP con archivos .md
Cuando POST /api/v1/imports con `{"format":"markdown-vault","project_id":"X"}` + multipart file
Entonces se procesa async
Y cada .md se importa como `knowledge_docs` con title del filename
Y front matter YAML se mapea a tags
Y links `[[Other]]` se preservan como referencias
Y se devuelve job_id para polling
```

### Escenario 2: Importer JSON dump

```gherkin
Dado que cargo JSON con estructura
  `{"observations":[...], "prompts":[...], "knowledge_docs":[...]}`
Cuando importo con `{"format":"json-dump"}`
Entonces cada entidad se inserta respetando IDs si existen y son únicos
Y conflictos por slug existente se resuelven con sufijo `-1`, `-2`
```

### Escenario 3: Importer Notion export

```gherkin
Dado que cargo ZIP export de Notion
Cuando importo con `{"format":"notion"}`
Entonces se parsean páginas .html y attachments
Y bloques se mapean a markdown
Y database CSV se mapea a knowledge_docs
```

### Escenario 4: Importer Obsidian vault

```gherkin
Dado que cargo ZIP de vault Obsidian
Cuando importo con `{"format":"obsidian"}`
Entonces backlinks `[[ref]]` se preservan
Y tags `#tag` se extraen
Y dataview blocks se ignoran con warning
```

### Escenario 5: Reporte de import

```gherkin
Dado que terminó un import
Cuando GET /api/v1/imports/:job_id
Entonces devuelve status, counts (created/skipped/failed), errors[], duration_ms
Y errors[] tiene path/línea para items fallados
```

### Escenario 6: Idempotencia

```gherkin
Dado que repito el mismo import
Cuando se procesa
Entonces items ya importados (por hash) se skip
Y solo se crean nuevos
```

## Análisis breve

- **Qué pide:** plug-and-play importers + job tracking + reporte
- **Esfuerzo:** L
- **Riesgos:** parsing variado por formato; performance en imports grandes; deduplicación correcta
