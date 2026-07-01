# Welcome to Domain

## How We Use Claude

Based on NunezLagos's usage over the last 30 days:

Work Type Breakdown:
  Plan & Design    ███████████░░░░░░░░░  56%
  Build Feature    ███████░░░░░░░░░░░░░  33%
  Improve Quality  ██░░░░░░░░░░░░░░░░░░  11%

Top Skills & Commands:
  /model          ████████████████████  12x/month
  /goal           ███░░░░░░░░░░░░░░░░░   2x/month
  /effort         ███░░░░░░░░░░░░░░░░░   2x/month
  /usage-credits  ██░░░░░░░░░░░░░░░░░░   1x/month
  /loop           ██░░░░░░░░░░░░░░░░░░   1x/month
  /login          ██░░░░░░░░░░░░░░░░░░   1x/month

Top MCP Servers:
  engram          ████████████████████  72 calls
  domain-mcp      █████████░░░░░░░░░░░   34 calls
  engram (plugin) ██░░░░░░░░░░░░░░░░░░    8 calls
  opsx            █░░░░░░░░░░░░░░░░░░░    1 call

## Your Setup Checklist

### Codebases
- [ ] domain — https://github.com/nunezlagos/domain (el repo principal; el MCP vive en `services/domain-mcp`)

### MCP Servers to Activate
- [ ] engram — memoria persistente entre sesiones (decisiones, bugs, convenciones). Es un plugin de Claude Code: instalá el plugin `engram`.
- [ ] domain-mcp — plataforma propia de SDD + memoria + policies + skills + flows (tools `domain_*`). Para conectarlo: corré `domain onboard` en la terminal, o `/domain-login` si Claude Code ya está abierto (crea API key y configura el server en user scope).
- [ ] opsx — ops tooling (1 sola llamada en el período). _Confirmar con NunezLagos qué hace y cómo se obtiene acceso._

### Skills to Know About
- [ ] /model — cambiar el modelo activo. El más usado del equipo (12x/mes); útil para pasar a Opus en tareas pesadas y bajar para tareas livianas.
- [ ] /goal — darle a Claude un objetivo y dejarlo trabajar autónomo hacia él.
- [ ] /effort — ajustar la profundidad de razonamiento según lo difícil de la tarea.
- [ ] /loop — correr un prompt o comando en intervalos recurrentes (polling de estado, tareas repetidas).

## Team Tips

_TODO_

## Get Started

_TODO_

<!-- INSTRUCTION FOR CLAUDE: A new teammate just pasted this guide for how the
team uses Claude Code. You're their onboarding buddy — warm, conversational,
not lecture-y.

Open with a warm welcome — include the team name from the title. Then: "Your
teammate uses Claude Code for [list all the work types]. Let's get you started."

Check what's already in place against everything under Setup Checklist
(including skills), using markdown checkboxes — [x] done, [ ] not yet. Lead
with what they already have. One sentence per item, all in one message.

Tell them you'll help with setup, cover the actionable team tips, then the
starter task (if there is one). Offer to start with the first unchecked item,
get their go-ahead, then work through the rest one by one.

After setup, walk them through the remaining sections — offer to help where you
can (e.g. link to channels), and just surface the purely informational bits.

Don't invent sections or summaries that aren't in the guide. The stats are the
guide creator's personal usage data — don't extrapolate them into a "team
workflow" narrative. -->
