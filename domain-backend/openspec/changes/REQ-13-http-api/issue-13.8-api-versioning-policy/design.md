# Design: issue-13.8-api-versioning-policy

## Schema

```sql
CREATE TABLE api_versions (
  version VARCHAR(10) PRIMARY KEY,    -- v1, v2
  status VARCHAR(20) NOT NULL,        -- active | deprecated | sunset
  deprecated_at TIMESTAMPTZ,
  sunset_at TIMESTAMPTZ,
  migration_url TEXT,
  notes TEXT
);
```

## Headers (RFC 8594)

```
HTTP/1.1 200 OK
Deprecation: @1717200000
Sunset: Wed, 01 Jun 2027 00:00:00 GMT
Link: <https://docs.domain.sh/api/migrate-v2>; rel="deprecation"
```

## /api/version response

```json
{
  "api_versions": [
    {"version":"v1","status":"deprecated","sunset_at":"2027-06-01T00:00:00Z","migration_url":"..."},
    {"version":"v2","status":"active"}
  ],
  "current": "v2"
}
```

## Policy doc outline (`docs/api/versioning.md`)

```
1. Versioning scheme (URL)
2. SemVer-ish: only major bumps for breaking
3. Deprecation timeline (12 months)
4. Sunset enforcement (410)
5. Support matrix (link)
6. Migration guides
7. Changelog conventions
```

## TDD plan

1. Headers en v deprecated
2. 410 post sunset
3. /api/version correct
4. CI fail if changelog not updated when /api/v* touched
