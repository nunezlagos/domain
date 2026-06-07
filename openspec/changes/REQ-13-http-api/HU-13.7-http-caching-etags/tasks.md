# Tasks: HU-13.7-http-caching-etags

- [ ] **et-001**: Helper `pkg/http/etag.go` compute/match
- [ ] **et-002**: Middleware ETag for GET single
- [ ] **et-003**: Cache-Control policy registry per route
- [ ] **et-004**: If-Match validation in PATCH/DELETE
- [ ] **et-005**: Last-Modified header
- [ ] **et-006**: Linter: GET single must declare cache policy
- [ ] **test-001**: ETag presente
- [ ] **test-002**: 304 If-None-Match
- [ ] **test-003**: PATCH 412 If-Match mismatch
- [ ] **test-004**: Cache-Control por route
- [ ] **test-005**: ETag changes on update
- [ ] **docs-001**: `docs/api/caching.md`
