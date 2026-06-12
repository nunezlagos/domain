# Tasks: HU-28.3-middleware-principal-crossorg

- [ ] **mc-001**: Definir `ctxkeys` package con `OrgIDKey`, `UserIDKey`
- [ ] **mc-002**: Implementar middleware `principal.Middleware` (post-auth, extrae Principal, parsea UUID, inyecta en ctx)
- [ ] **mc-003**: Agregar helpers `orgID(ctx)`, `userID(ctx)`, `authorizeOrg(ctx, resourceOrgID)` en `handler/api.go`
- [ ] **mc-004**: Insertar middleware en la chain de `main.go` (después de auth middleware)
- [ ] **mc-005**: Migrar 5 handlers: observation, session, flow, agent, project
- [ ] **mc-006**: Reemplazar cross-org guard manual por `authorizeOrg` en los 5 handlers migrados
- [ ] **mc-007**: Tests unitarios: middleware inyecta OrgID correcto, authorizeOrg bloquea cross-org
- [ ] **mc-008**: Suite completa verde
