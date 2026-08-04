#!/bin/bash
# PreToolUse [Bash] scoped al subagente git-archaeology (DOMAINSERV-139).
#
# El campo `tools` del frontmatter de un subagente NO soporta sintaxis
# `Bash(git log:*)` para acotar subcomandos — se verificó contra la doc oficial
# (code.claude.com/docs/en/sub-agents, sección "Available tools"): tools solo
# resuelve nombres de tool completos (Bash, Read, ...) o patrones
# `Agent(tipo)` / `mcp__server`. La tabla de DOMAINSERV-139 declaraba
# `Bash(git log:*/show:*/blame:*)` como si fuera allowlist de frontmatter;
# no lo es. El mecanismo real, documentado en la misma sección ("Conditional
# rules with hooks", ejemplo db-reader), es `tools: Bash` + este hook.
#
# Complementa — no reemplaza — el git-guard global de domain-pre-edit.sh: ese
# bloquea comandos destructivos conocidos (reset --hard, clean, stash, checkout
# --, restore, rm, worktree remove) para CUALQUIER Bash, incluso subagentes
# (DOMAINSERV-134). Este hook es más estricto: allowlist, no denylist — solo
# git log / show / blame pasan, todo lo demás se bloquea.
payload=$(cat)

if [ -n "$payload" ] && ! command -v python3 >/dev/null 2>&1; then
  echo '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"git-archaeology-guard: python3 no disponible, fail-closed."}}'
  exit 0
fi

eval "$(printf '%s' "$payload" | python3 -c '
import json, sys, shlex
try:
    d = json.load(sys.stdin)
except Exception:
    print("parse_failed=yes")
    sys.exit(0)
ti = d.get("tool_input") or {}
print("tool_cmd=%s" % shlex.quote(ti.get("command", "") if isinstance(ti, dict) else ""))
' 2>/dev/null)"

if [ "${parse_failed:-}" = "yes" ]; then
  echo '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"git-archaeology-guard: payload no parseable, fail-closed."}}'
  exit 0
fi

decision=$(printf '%s' "$tool_cmd" | python3 -c '
import re, sys
cmd = sys.stdin.read().strip()

# Un solo comando git simple: nada de encadenado, pipes, subshells, sustitución
# ni redirección. El allowlist de abajo solo tiene sentido si evalúa UN comando.
if re.search(r"[;&|`\n]|\$\(|<\(|>\(", cmd):
    print("deny:encadenado o con subshell/sustitución — solo se permite un comando git simple")
    sys.exit(0)
if re.search(r"[<>]", cmd):
    print("deny:redirección de entrada/salida no permitida")
    sys.exit(0)

# opciones/variables que redirigen git a OTRO repo/directorio se rechazan
# categóricamente — no se "despojan y se evalúa igual": el punto de este
# allowlist es que corra sobre el repo actual, no sobre cualquier ruta legible.
if re.search(r"\bGIT_(?:DIR|WORK_TREE|COMMON_DIR)\s*=", cmd):
    print("deny:variables GIT_DIR/GIT_WORK_TREE/GIT_COMMON_DIR no permitidas")
    sys.exit(0)
if re.search(r"\bgit\s+(?:-[cC]\b|--git-dir\b|--work-tree\b)", cmd):
    print("deny:opciones globales de git (-C/-c/--git-dir/--work-tree) no permitidas")
    sys.exit(0)

# --output es de la familia diff, la comparten log y show, y ESCRIBE el archivo sin
# pasar por el operador de redirección: la rama de [<>] de arriba no lo ve y el
# allowlist de abajo lo acepta por empezar con "git log". Era una primitiva de
# escritura arbitraria en el agente de menor riesgo del catálogo. El límite exige
# "=", espacio o fin para no pisar --output-indicator-*, que solo cambia el marcador
# del diff y no escribe nada.
if re.search(r"(?:^|\s)--output(?:=|\s|$)", cmd):
    print("deny:--output escribe un archivo — este agente no muta el filesystem")
    sys.exit(0)

if re.match(r"^\s*git\s+(log|show|blame)\b", cmd):
    print("allow:")
else:
    print("deny:solo se permite git log / git show / git blame")
')

decision_kind="${decision%%:*}"
decision_reason="${decision#*:}"

[ "$decision_kind" = "allow" ] && exit 0

python3 -c '
import json, sys
print(json.dumps({"hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": sys.argv[1],
}}))
' "git-archaeology-guard: $decision_reason. Este agente es Bash acotado a git log/show/blame (DOMAINSERV-139); cualquier otro comando lo corre el hilo principal."
exit 0
