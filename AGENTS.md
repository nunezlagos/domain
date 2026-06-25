# Domain — Project AGENTS.md

Override del `~/.config/opencode/AGENTS.md` global.

> **IMPORTANTE:** Este proyecto se construye 100% con agentes IA dirigidos por humanos. Antes de tocar nada, leé `.claude/rules/ai-generation.md` para entender el workflow.

## Stack
- **Lenguaje:** Go 1.22+
- **DB:** Postgres 15+ con pgvector, tsvector (vía pgx v5)
- **CLI:** Cobra + Viper
- **MCP:** mark3labs/mcp-go
- **Migraciones:** golang-migrate
- **Tests:** testing + testify + testcontainers-go
- **Logging:** slog

## SDD First
Todo cambio requiere HU. Las HUs están en `openspec/changes/`. Si no existe la HU, no se implementa.

## No tocar
- `openspec/` — documentación SDD, no código
- `.claude/` y `AGENTS.md` — config del agente
- Archivos archivados en `openspec/changes/archive/`

## Código — reglas de calidad

### Comentarios
- **El código debe ser auto-descriptivo.** Nombres claros eliminan la necesidad de comentar.
- **Solo agregar un comentario cuando es estrictamente necesario**: workaround, restricción externa no evidente, decisión que sorprendería a quien lee el código.
- Si sentís que necesitás comentar el QUÉ, es señal de que el nombre está mal — renombrá, no comentés.
- **Siempre en la línea anterior** al código. Nunca al final de la misma línea.
- Frase corta, puntual, minúscula, sin punto final.
- Sin bloques multi-línea, sin secciones decorativas, sin docstrings largos.

### Tamaño de funciones
- **Máximo 50 líneas por función** (sin contar la firma ni el cierre `}`).
- Si una función supera 50 líneas: extraé responsabilidades a funciones con nombre semántico.
- Las únicas excepciones aceptables son funciones de wiring/DI en `main` o `server.go` — documentadas con un comentario de una línea explicando por qué no se puede partir.
- Una función larga es una señal de SRP violado, no de complejidad necesaria.

### Acoplamiento
- Las interfaces se definen en el **consumidor**, no junto a la implementación.
- Interfaces chicas (1-3 métodos). Una interfaz de 10+ métodos es un God Interface.
- Los handlers y tools MCP nunca dependen de tipos concretos de service — siempre contra interfaz.

## Recordatorio
- Tools MCP con prefijo `domain_`
- CLI con `domain` (no `memoria`)
- Env vars con `DOMAIN_`
- NO agregar Co-Authored-By en commits
- NO hacer build después de cambios
