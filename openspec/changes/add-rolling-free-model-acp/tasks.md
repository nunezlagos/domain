# Tasks — add-rolling-free-model-acp

- [ ] 1. (code) `rolling.go`: tipo `roller` con roster, índice round-robin (mutex), cooldown map, home por modelo. Done = compila.
- [ ] 2. (code) `rolling.go`: `discover()` — parsear `opencode models --verbose`, filtrar cost==0, cachear último roster; `refresh()` background TTL.
- [ ] 3. (code) `provider.go` + `register.go`: `Complete` usa `roller.next()` + HOME por modelo; cablear config por env.
- [ ] 4. (code) `process.go`: dedup de HOME en `scrubbedEnv` (last-wins) si el HOME de cfg.Env no gana.
- [ ] 5. (tests) `rolling_test.go`: round-robin, cooldown, discover-vacío-conserva-roster, parse-cost-cero, dedup-HOME.
- [ ] 6. (sabotage) por cada test, aplicar el sabotaje del plan y confirmar que falla.
- [ ] 7. (verify) auditar: archivos nuevos <150 líneas, sin secrets, sin N+1, `go test ./internal/llm/acp/...` verde.
