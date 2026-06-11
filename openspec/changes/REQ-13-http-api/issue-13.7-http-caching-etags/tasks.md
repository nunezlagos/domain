# Tasks: issue-13.7-http-caching-etags

- [x] **et-001**: Helper `pkg/http/etag.go` compute/match
- [x] **et-002**: Middleware ETag for GET single
- [x] **et-003**: Cache-Control policy registry per route
- [x] **et-004**: If-Match validation in PATCH/DELETE
- [x] **et-005**: Last-Modified header
- [x] **et-006**: Linter: GET single must declare cache policy
- [x] **test-001**: ETag presente
- [x] **test-002**: 304 If-None-Match
- [x] **test-003**: PATCH 412 If-Match mismatch
- [x] **test-004**: Cache-Control por route
- [x] **test-005**: ETag changes on update
- [x] **docs-001**: `docs/api/caching.md`
