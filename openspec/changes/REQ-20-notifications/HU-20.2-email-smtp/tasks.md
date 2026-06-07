# Tasks: HU-20.2-email-smtp

- [ ] **smtp-001**: Dep `github.com/wneessen/go-mail`
- [ ] **smtp-002**: `channels/email/channel.go` implements Channel
- [ ] **smtp-003**: Config struct + vars `DOMAIN_SMTP_*` en HU-01.2
- [ ] **smtp-004**: DKIM signing opcional
- [ ] **smtp-005**: Validador email
- [ ] **smtp-006**: Register en notifications registry al boot
- [ ] **test-001**: Unit mock SMTP server
- [ ] **test-002**: Integration Mailpit dev compose
- [ ] **test-003**: Email inválido fail-fast no retry
- [ ] **test-004**: DKIM header presente
- [ ] **docs-001**: `docs/notifications/email.md` setup SPF/DKIM/DMARC en prod
