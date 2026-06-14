# Tasks: issue-20.3-slack-webhook

- [ ] **wh-001**: HTTP client compartido `channels/webhook/http.go`
- [ ] **wh-002**: SlackChannel con Block Kit
- [ ] **wh-003**: GenericChannel con headers + HMAC
- [ ] **wh-004**: Rate limiter token bucket por recipient
- [ ] **wh-005**: URL redactor para logs
- [ ] **wh-006**: Cifrado recipient en DB via issue-02.3
- [ ] **wh-007**: Register en notifications registry al boot
- [ ] **test-001**: httptest body + headers
- [ ] **test-002**: Block Kit JSON schema valid
- [ ] **test-003**: Rate limit 10→1/s
- [ ] **test-004**: HMAC verify
- [ ] **test-005**: URL no https → reject
- [ ] **docs-001**: `docs/notifications/slack.md` setup webhook URL
