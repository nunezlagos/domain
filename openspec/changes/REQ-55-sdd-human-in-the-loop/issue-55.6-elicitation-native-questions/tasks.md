# issue-55.6 elicitation-native-questions — Tasks
- [ ] handler.go: WithStateLess(false) + WithSessionIdleTTL(30m)
- [ ] server.go: WithElicitation() en NewMCPServer
- [ ] tool/paso que dispara srv.RequestElicitation con schema enum+texto (punto de disparo en spec)
- [ ] Fallback: ErrElicitationNotSupported → gate user_answers
- [ ] Tests Go: sesión soporta → dispara; no soporta → fallback
- [ ] Verificar concurrencia (2+ sesiones no se pisan) — reusar aprendizaje del probe
- [ ] Deploy VPS + verificación en vivo con el cliente real
