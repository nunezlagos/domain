# Design: issue-23.3-gdpr-export

## Schema

```sql
CREATE TABLE export_jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  progress_percent SMALLINT DEFAULT 0,
  s3_key VARCHAR(500),
  size_bytes BIGINT,
  signed_url TEXT,
  signed_url_expires_at TIMESTAMPTZ,
  error TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  finished_at TIMESTAMPTZ
);
CREATE INDEX ON export_jobs (user_id, created_at DESC);
```

## Estructura del ZIP

```
export.zip
├── manifest.json          # version, export_date, user_id, file_list
├── README.md              # explicación
├── profile.json
├── organizations.json
├── projects.json
├── observations.json
├── sessions.json
├── prompts.json
├── knowledge_docs.json
├── agent_runs.json
├── flow_runs.json
└── attachments/
    ├── <uuid1>.png
    └── <uuid2>.pdf
```

## Components

```
internal/service/export.go
internal/exporter/
  exporter.go       # orchestrator
  serializer.go     # JSON streaming
  zipper.go         # ZIP streaming
  s3uploader.go
```

## TDD plan

1. User con fixture data → ZIP válido + claves esperadas
2. Adversarial: user A export NO incluye datos user B
3. Rate-limit 2do request 429
4. Crash mid-job → status failed con error
5. Signed URL accesible solo durante TTL
