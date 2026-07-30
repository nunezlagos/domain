# Acotar el cuerpo por item en los tools MCP de listado y búsqueda — Tasks

DOMAINSERV-161 · issue-54.1

Cinco bloques de entrega en orden de riesgo creciente. Las dos restricciones duras del ticket se respetan en los bloques 4 y 5: hook y server en el mismo commit, y gate más bump del seeder en el mismo commit.

## Bloque 1 — Molde compartido (grupos 1-2)

- [ ] **1. code** · grupo 1 · 2h — Crear el DTO slim genérico para los 4 structs sin tags json (`Skill`, `Agent`, `Flow`, `SearchResult`), clonando el idiom de `ticket_slim_dto.go`: `campoOmitido` + embedding con shadowing + campo `*Len`, preservando PascalCase. Done cuando compila y `proyectar*ParaListado` existe para los 4.
- [ ] **2. tests** · grupo 2 · 1h — Escribir `TestSkillSlim_Proyectar_ContentLargo_OmiteCuerpoYExponeLen` verificando ausencia de la clave, ausencia de la subcadena del cuerpo y `ContentLen` con valor literal exacto. Done cuando falla en rojo antes de cablear el handler.

## Bloque 2 — Las 6 tools sin consumidor delicado (grupos 3-4)

- [ ] **3. code** · grupo 3 · 1h — Cablear la proyección en `skill_inline.go` para `domain_skill_list` y `domain_skill_search`. Done cuando pasa el test 2.
- [ ] **4. code** · grupo 3 · 1h — Cablear la proyección en `agent_inline.go` (`SystemPrompt` fuera) y `flow_inline.go` (`Spec` fuera). Archivos distintos del 3, corren en paralelo.
- [ ] **5. code** · grupo 3 · 1h — Cablear `captured_prompt_tools.go` reusando el `CharCount` que YA existe, sin inventar un campo nuevo, y `policy_tools.go` conservando `slug`, `name`, `kind`, `version` y `override_platform`, porque el prompt de sdd-review depende del último.
- [ ] **6. tests** · grupo 4 · 2h — Un test de shape por cada una de las 6 tools, con el mecanismo del bloque 1. Done cuando los 6 pasan y ninguno tolera el cuerpo.

## Bloque 3 — Guards (grupos 5-6)

- [ ] **7. code** · grupo 5 · 2h — Insertar el guard de bytes en `resilience.go:340`, justo después de `execWithRetry` y antes del cache. Debe truncar sobre el valor antes de serializar y emitir envelope JSON válido con `truncated=true`, nunca cortar la cadena JSON final.
- [ ] **8. code** · grupo 5 · 2h — Crear el binario de lint nuevo bajo `cmd/`, reusando de `response-shape-lint` solo el esqueleto (parseo `go/ast`, tipo `Violation`, exit codes) y detectando `&mcp.CallToolResult{}` en `internal/mcp/server`. Sin baseline de excepciones.
- [ ] **9. code** · grupo 5 · 1h — Normalizar los 17 sitios manuales a `toolResultJSON`: `orchestrate_tools.go` (9), `workflow_tools.go` (4), `workflow_trace_tools.go` (3), `prompt_tools.go` (1). Done cuando el linter del paso 8 reporta cero.
- [ ] **10. tests** · grupo 6 · 2h — `TestResilientWrapper_Wrap_PayloadSobreLimite_DevuelveJSONValidoConTruncated`, `TestResilientWrapper_Wrap_EscapeUnicodeEnElBorde_NoRompeElParseo`, `TestMCPResultLint_CallToolResultManual_ReportaViolacion` y `TestMCPResultLint_RepoActual_CeroViolaciones`.
- [ ] **11. docs** · grupo 6 · 1h — Agregar el job del linter nuevo a `ci-mcp.yml`, siguiendo el patrón del job existente en las líneas 75-87.

## Bloque 4 — Búsquedas y hook, MISMO COMMIT (grupos 7-8)

- [ ] **12. code** · grupo 7 · 2h — Truncar a 200 con el helper `truncate` existente en `memory_inline.go` (`mem_search` y `mem_context`) y `knowledge_inline.go`, conservando el nombre de key actual y agregando `content_len`. PROHIBIDO redeclarar `truncate` y PROHIBIDO renombrar la key a `snippet`.
- [ ] **13. code** · grupo 7 · 1h — Actualizar `install-user/hooks/domain-user-prompt.sh` y `install-user/templates/agents/domain-memory.md` para que el prompt del agente nombre el `*_len` como criterio de expansión. Va en el MISMO commit que el paso 12.
- [ ] **14. tests** · grupo 8 · 2h — `TestMemSearch_Handler_ContentLargo_TruncaA200YReportaLen` y `TestMemContext_Handler_MismoShapeQueMemSearch`, más el primer test que ejercite el parseo real del hook, que hoy no tiene ninguno (`domain-session-start_host_test.sh:27` mockea `HOOK_MEM_OUT=""`).

## Bloque 5 — El gate, MISMO COMMIT que el bump (grupo 9)

- [ ] **15. code** · grupo 9 · 2h — Reescribir el system_prompt seedeado de `sdd-review` (`agent_templates_catalog.go:836`) para fan-out de `domain_policy_get` por slug, sumar `domain_policy_get` a `RequiredToolCalls` (`sdd_review.go:79`), y bumpear `agentTemplatesSeedVersion` de 20 a 21 (`:1090`) EN EL MISMO COMMIT.
- [ ] **16. code** · grupo 9 · 1h — Actualizar la policy `context-preservation` (`platform_policies_seeder.go:474`) para que re-hidrate con `domain_mem_get_observation` por id, con el bump del seeder de platform policies.

## Sabotajes (grupo 10)

- [ ] **17. sabotage** · grupo 10 · 2h — Ejecutar los 9 sabotajes del TDD plan, uno por uno, confirmando que cada test falla por la razón correcta y restaurando después. Método: conteo de ocurrencias antes y después con la ventana acotada al fragmento que gobierna el comportamiento, porque hay 5 casos registrados de falso positivo por subcadena en este repo. Un error de setup es un fallo, nunca un skip.
- [ ] **18. sabotage** · grupo 10 · 1h — Verificación POR EJECUCIÓN del gate: correr `sdd-review` con el shape nuevo sobre un diff que viola una policy conocida y confirmar que el verdict sale `violations_found` y bloquea. Hoy, con slugs sin cuerpo, devuelve `compliant`.

## Verify (grupo 11)

- [ ] **19. verify** · grupo 11 · 1h — Auditar el change completo: ninguna función nueva supera 50 líneas, inputs validados en los boundaries, sin secretos hardcodeados, sin N+1 nuevas, sin código muerto, y las DOS suites corridas a mano (`go test ./...` en `services/domain-mcp` y en `install-user`), porque el filtro de paths de `ci-mcp.yml` no dispara la de install-user.
