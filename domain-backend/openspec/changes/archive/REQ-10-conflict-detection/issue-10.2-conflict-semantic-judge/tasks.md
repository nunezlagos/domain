# Tasks: issue-10.2-conflict-semantic-judge

## Backend

- [ ] **B1: Implementar AgentCLI interface y resolveAgent**
      - `internal/conflict/semantic.go`
      - ClaudeAgent — exec "claude -p {prompt}"
      - OpenCodeAgent — exec "opencode prompt {prompt}"
      - Factory: resolveAgent(agentName string) AgentCLI

- [ ] **B2: Crear prompt template estructurado**
      - System prompt con definiciones de verdict
      - Source + target content contextualizado
      - JSON response format specification

- [ ] **B3: Implementar parseJudgment**
      - Extraer JSON del output (puede tener texto adicional)
      - Validar verdict contra set conocido
      - Extraer confidence y reason

- [ ] **B4: Implementar judgeOne**
      - Construir prompt con source y target
      - Ejecutar AgentCLI.Judge con timeout
      - Persistir resultado en memory_relations
      - Manejar errores: timeout, exec, invalid_verdict

- [ ] **B5: Implementar JudgePending con worker pool**
      - Concurrency via buffered channel semáforo
      - MaxJudgments limit
      - Collect results + report

- [ ] **B6: Implementar updateJudgmentError**
      - Set judgment_status = "error", reason con descripción

- [ ] **B7: Implementar skip de already judged**
      - Query solo WHERE judgment_status = 'pending'

- [ ] **B8: Implementar ENGRAM_AGENT_CLI env var**
      - Leer al inicio; default "claude"

## Tests

- [ ] **T1: parseJudgment parsea JSON válido**
      ```go
      func TestParseJudgment(t *testing.T) {
          j, err := parseJudgment(`{"verdict":"duplicate","confidence":0.95,"reason":"same content"}`)
          assert.NoError(t, err)
          assert.Equal(t, "duplicate", j.Verdict)
      }
      ```

- [ ] **T2: parseJudgment falla con verdict inválido**
- [ ] **T3: parseJudgment extrae JSON de texto adicional**
      ```go
      func TestParseJudgmentExtraText(t *testing.T) {
          j, err := parseJudgment("Here is my analysis:\n{\"verdict\":\"supersedes\",...}\nHope this helps")
          assert.NoError(t, err)
          assert.Equal(t, "supersedes", j.Verdict)
      }
      ```

- [ ] **T4: judgeOne persiste verdict en DB (mock agent)**
- [ ] **T5: judgeOne error → judgment_status = "error"**
- [ ] **T6: JudgePending procesa con concurrencia limitada**
- [ ] **T7: MaxJudgments limita procesamiento**
- [ ] **T8: Skip already judged**
- [ ] **T9: No pending → mensaje informativo**
- [ ] **T10: Sabotaje — no validar verdict → test T2 falla → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/conflict/... -v`
- [ ] Commit: `feat: semantic LLM judge for conflict detection`
