# Design: issue-13.4-idempotency-keys

## Schema

```sql
CREATE TABLE idempotency_records (
  key VARCHAR(255) NOT NULL,
  organization_id UUID NOT NULL REFERENCES organizations(id),
  request_hash BYTEA NOT NULL,
  request_method VARCHAR(10) NOT NULL,
  request_path VARCHAR(500) NOT NULL,
  response_status INT NOT NULL,
  response_headers JSONB,
  response_body BYTEA,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  PRIMARY KEY (organization_id, key)
);
CREATE INDEX ON idempotency_records (expires_at);
```

## Middleware flow

```
1. method in [POST,PATCH,DELETE] AND header Idempotency-Key present?
   no → pass through
2. orgID = ctx auth
3. hash = sha256(method+path+body)
4. tx:
   SELECT FOR UPDATE record WHERE (org, key)
   if exists AND not expired:
     if hash matches → return cached response
     else → 422 conflict
   else:
     INSERT placeholder (response_status=0)
5. call next handler, capture response via wrapped ResponseWriter
6. UPDATE record with response (status,body,headers) + expires_at now+24h
7. tx commit
8. write response to client
```

## TDD plan

1. First request stores + responds
2. Replay cached + header `Idempotent-Replayed: true`
3. Body mismatch → 422
4. Concurrent (2 goroutines) → 1 ejecuta, 1 cachea
5. TTL expired → reprocess
6. GET no aplica
7. 5xx no se cachea (puede reintentar)
