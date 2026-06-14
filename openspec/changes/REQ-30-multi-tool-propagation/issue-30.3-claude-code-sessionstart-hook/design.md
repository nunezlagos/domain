# Design: issue-30.3-claude-code-sessionstart-hook

## Contexto

Claude Code expone `SessionStart` hook en
`~/.claude/settings.json` que ejecuta commands arbitrarios al abrir
cada sesión. Es la vía oficial para integrar side effects pre-sesión
(auto-format, linters, project bootstrap, etc.). La documentamos como
feature estándar de Claude Code — no estamos inventando un sidecar.

La integración con domain es: ejecutar `domain setup auto-detect
"$PWD" --quiet` en SessionStart. El setup es idempotente y O(1) cuando
no hay cambios, así que el overhead por sesión es despreciable.

## Decisión arquitectónica

**Estrategia:** merge de `hooks.SessionStart` con diff + confirm.

1. **Path:** `~/.claude/settings.json` (convención Claude Code; en
   macOS también es `~/Library/Application Support/Claude/settings.json`
   pero la convención de dotfile es lo standard).
2. **Command canónico:**
   ```json
   {
     "type": "command",
     "command": "domain setup auto-detect \"$PWD\" --quiet"
   }
   ```
3. **Merge strategy:** append al array `hooks.SessionStart`. Si el
   array no existe, crearlo. Si la key `hooks` no existe, crearla con
   solo SessionStart. Cualquier OTRA key del settings.json (tema,
   permisos, otros hooks) se preserva intacta.
4. **Diff + confirm:** install muestra el JSON resultante antes de
   escribir. Si el user rechaza, no se modifica nada.
5. **Backup:** antes de escribir, respaldar `settings.json` con
   `install.BackupFile`. Nombre del backup: `settings.json.bak-<ts>`.
6. **Idempotencia:** antes de mergear, buscar en el array existente un
   command que matchee `domain setup auto-detect`. Si está, skip
   (no duplicar).
7. **Permisos:** chmod 600 al `settings.json` (puede llevar API keys
   según otras configs del user).

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Hook `UserPromptSubmit` (en vez de SessionStart) | Tarda más: el primer prompt del user ya disparó antes del hook. SessionStart corre antes de que el agente lea nada. |
| B | Sobrescribir `settings.json` entero | Destruye configs del user (permisos, otros hooks, tema). Merge es seguro. |
| C | Hook en `~/.claude.json` (config de proyecto) | El path canónico es `settings.json`. Claude Code no lee hooks de `claude.json` (es config de proyecto, no del user). |
| D | Sidecar script `~/.claude/hooks/domain.sh` | Claude Code soporta hooks inline en settings.json. Un sidecar agrega indirección sin beneficio. |

## Por qué merge + diff gana

- **No destructivo:** el user ve el diff antes de aceptar. Si
  rechaza, settings.json queda intacto.
- **Compatible:** merge preserva hooks del user.
- **Auditable:** el manifest global (REQ-30.4) registra la
  operación con timestamp + path + antes/después hash.
- **Reversible:** uninstall (REQ-30.4) puede remover el hook del
  array con la misma lógica de diff inverso.

## Detalle de implementación

Paquete `internal/cli/setup/claudehook/`:

- `claudehook.go` — funciones:
  - `ReadSettings() (*Settings, error)` — lee + parsea
    `~/.claude/settings.json`. Tolerante a JSON malformado (loggea
    warning + retorna struct vacío).
  - `HasDomainHook(s *Settings) bool` — busca en
    `s.Hooks.SessionStart` un command que matchee
    `regexp.MustCompile(\`^domain setup auto-detect\`)`.
  - `AddDomainHook(s *Settings) *Settings` — append al array (inmut;
    retorna nuevo struct).
  - `DiffSettings(before, after *Settings) string` — retorna diff
    human-readable (qué se agregó).
  - `InstallClaudeHook(nonInteractive bool) (installed bool,
    changed bool, err error)` — entry: lee, calcula diff, muestra
    (si non-interactive salta el prompt), escribe con backup.

Wiring en `runInstall`: nuevo step "Claude Code SessionStart hook"
entre wrapper (30.2) y init.

## Riesgos

- **R1:** Settings.json malformado (sintaxis JSON rota por edición
  manual). **Mitigación:** `ReadSettings` retorna warning + struct
  vacío, pero NO sobrescribe sin confirm. Mejor perder el hook que
  perder el settings.json entero.
- **R2:** `domain` no en PATH al disparar el hook. **Mitigación:** el
  command es `domain setup auto-detect ...`. Si domain está en
  `/usr/local/bin` o `~/.local/bin` (post-install), está en PATH. Si
  el user mueve el binario, debe ajustar el hook — se documenta en
  README.
- **R3:** Confirm prompt puede romper scripts no-interactive. **Mitigación:**
  flag `--with-claude-hook` (yes) o `--no-claude-hook` (no) para
  controlar sin prompt.

## Sabotaje test (referencia)

Cambiar el command del hook a `echo noop` (mantener estructura JSON)
→ test que assserta "manifest existe post-hook" DEBE FALLAR →
restaurar → test verde.
