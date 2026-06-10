# Security Conventions — Domain

Regla de oro: **datos sensibles NUNCA en logs, métricas, traces, ni responses que no sean explícitamente para revelar ese dato**.

## Datos sensibles (lista cerrada)

| categoría | ejemplos | tratamiento |
|-----------|----------|-------------|
| Auth secrets | api_key plaintext, password, otp_code, jwt | NUNCA log/trace/metric; revelar UNA vez en endpoint específico |
| PII identidad | email, rut, name, phone, address, dob | redact en logs; mostrar solo a self o con RBAC |
| Pagos | card pan, cvc, stripe customer secret | NUNCA tocar (delegado a Stripe Checkout issue-21.4) |
| Tokens externos | google oauth tokens, github tokens | cifrar at-rest (issue-02.3); no log |
| Contenido user | observation content, knowledge body | NO log full content (puede tener secrets); log metadata |
| Webhook URLs | hooks.slack.com/... | cifrar at-rest (issue-02.3); redact en logs |
| Encryption keys | master keys, JWT signing | en KMS/Vault, NUNCA en código ni env plain |

## Logging

### Keys prohibidas en `slog`

Lista bloqueada por linter (issue-17.3 linter test):

```
password, passwd, secret, token, key, api_key, apikey,
otp, otp_code, code, signature, hmac, jwt,
email, rut, phone, dob, address,
pan, cvc, card_number,
content, body, payload (cuando es body grande)
```

Si necesitás logear algo cercano: usar formas seguras:
- `email_hash` (sha256 first 8 chars)
- `key_prefix` (primeros 16 chars del API key)
- `user_id` (UUID, no email)
- `content_length` (no content)

### Niveles

- `Debug`: info granular dev/staging, OFF en prod por default
- `Info`: eventos importantes (request, run completed, login)
- `Warn`: condiciones recoverables (retry, degraded mode)
- `Error`: errores que requieren atención (failure no recoverable)

## Métricas

- Labels con cardinalidad acotada (no `user_id`, no `request_id`, no `run_id`)
- Linter issue-17.1 detecta `_id` regex en labels
- Counters: `_total` sufijo siempre
- Histograms: buckets razonables

## Traces (OpenTelemetry)

- Span attributes whitelist (issue-17.2 `SafeAttrs()`)
- NUNCA `attr.String("email", user.Email)` etc.
- Trace IDs OK en logs (no son sensibles)

## Secrets management

### En código
- NUNCA hardcodear secret
- NUNCA en `.env` committeado (`.env` está en `.gitignore`)
- `.env.example` con valores triviales/placeholders

### Runtime
- Secrets via K8s Secret (referenciados en env vars)
- Mejor: External Secrets Operator (ESO) syncing de AWS Secrets Manager / GCP / Vault
- Master encryption key NUNCA en plain env: KMS-managed con auto-decrypt

### Rotation
- API keys: rotables vía issue-02.7 (regenerate action)
- DB passwords: issue-25.10 cada 90 días
- Master encryption keys: anual con re-encrypt all (issue-02.3)
- OAuth secrets: si aplica futuro, rotación documentada

## Authentication & authorization

- API key auth: issue-02.1 — bcrypt cost 12 hash, prefix visible, plaintext solo en /auth/verify-otp response
- Sesiones: NO usamos sesiones server-side (la API key es el credential)
- RBAC: issue-02.2 + issue-02.8; SIEMPRE check en service layer ANTES de query
- RLS: issue-25.5 defense-in-depth para 12 tablas críticas
- Cross-org leak: bloqueado en app (RBAC) + en DB (RLS); test adversarial mandatorio

## Input validation

- TODO input externo (HTTP body, query params, MCP args) → validar antes de tocar DB
- JSON Schema validation para skills (issue-05.6) y APIs
- Whitelist mejor que blacklist
- SQL injection: pgx parameterized (NUNCA `fmt.Sprintf`); linter en issue-25.13
- XSS en respuestas: API es JSON puro, no HTML; pero si renderizamos email HTML (issue-20.2), escapar siempre
- Path traversal: NO usar input para construir paths filesystem; usar S3 keys con prefijos seguros

## CORS

- `/api/v1/*`: CORS deshabilitado por default (clientes son SDKs server-to-server)
- Si en algún momento hay browser client: lista explícita de origins permitidos

## Anti-enumeration

- 404 idéntico para "no existe" y "no autorizado"
- Login passwordless (issue-02.7): respuesta 200 idéntica aunque user no exista
- Timing constante: p99 ±5ms entre happy y not-found path

## Network

- DB accesible solo desde pods Domain (NetworkPolicy issue-24.1)
- PgBouncer solo accesible desde Domain pods
- Métricas `/metrics` bind 127.0.0.1 o auth required
- Webhooks outbound: SSRF prevention (issue-10.4)

## Backups y exports

- Backups Postgres cifrados (issue-18.1 pgBackRest `repo1-cipher-type=aes-256-cbc`)
- S3 backups con SSE-KMS (issue-18.2)
- GDPR export ZIP con signed URL 24h max (issue-23.3)
- Anonymization para staging dumps (issue-25.11)

## Threat model recordatorio

| amenaza | mitigación principal |
|---------|---------------------|
| SQL injection | parameterized queries + linter |
| Cross-org leak por bug RBAC | RLS en tablas críticas (issue-25.5) |
| API key leak | redact logs + rotation + prefix-only display |
| Stolen email → OTP login | invitación-only signup; admin notification on new login |
| Compromised pod → access prod DB | least-privilege roles (issue-25.6); no plaintext secrets |
| Malicious agent prompt | output validation skill (issue-05.6); budget caps |
| SSRF via webhook | URL validator (issue-10.4) |
| Replay attack (OTP, webhooks) | nonces, timestamps, single-use |
| DoS via expensive queries | statement_timeout (issue-25.8) + rate limit + plan quotas |

## Vulnerability disclosure

- `SECURITY.md` en raíz del repo
- Email para reportes con PGP key
- 90 days disclosure window
