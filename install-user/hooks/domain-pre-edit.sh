#!/usr/bin/env bash
# hooks/domain-pre-edit.sh — hook PreToolUse de Claude Code (Edit|Write|
# NotebookEdit|Bash).
#
# REQ-54 issue-54.7: gate determinista SDD-para-código. TODO código pasa por
# SDD (decisión del usuario, sin exención trivial):
#
#   - Si la sesión tiene flow SDD activo (marca de domain-post-orchestrate.sh)
#     → la edición pasa.
#   - Sin flow: en modo normal (default/plan) → permissionDecision "ask" (el
#     HUMANO decide en el diálogo); en modos automáticos (acceptEdits/
#     bypassPermissions/auto) → "deny" con razón: el agente es FORZADO a
#     orquestar primero.
#   - Bash solo se gatea si el comando PARECE edición de código (sed -i, tee,
#     patch, git apply, redirect, perl -i, python -c open(w), cp/mv, dd of=,
#     here-doc a archivo de código). Limitación conocida: heurística con
#     falsos negativos posibles — la policy explícita cubre el resto.
#
# Endurecimiento (pre-edit-hardening):
#   (A) GIT GUARD: git destructivo (reset --hard / clean / stash / checkout
#       -- | .) → deny SIEMPRE, incluso con flow activo o en subagentes.
#       Defensa en profundidad por si el permissions.deny fallara.
#   (B) heurística de edición ampliada (ver arriba) para atrapar bypass.
#   (C) COMMIT-GATE: git commit sin marker fresco de tests verificados →
#       ask (default/plan) o deny (modos automáticos).
#
# Best-effort en fallos de parseo: permitir (exit 0) antes que romper la sesión.
set +e

# Lib compartida (best-effort): aporta domain_log_injection. Si no está,
# el hook igual funciona — el logging es opcional, jamás bloquea.
LIB="$(dirname "$0")/domain-hooks-lib.sh"
[ -r "$LIB" ] && . "$LIB"

payload=$(cat)
eval "$(printf '%s' "$payload" | python3 -c '
import json, sys, shlex
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
ti = d.get("tool_input") or {}
print("session_id=%s" % shlex.quote(d.get("session_id", "")))
print("tool_name=%s" % shlex.quote(d.get("tool_name", "")))
print("perm_mode=%s" % shlex.quote(d.get("permission_mode", "default")))
print("tool_cmd=%s" % shlex.quote(ti.get("command", "") if isinstance(ti, dict) else ""))
' 2>/dev/null)"
[ -n "$session_id" ] || exit 0

# emit_decision <decision> <reason> — emite el permissionDecision y termina.
emit_decision() {
  type domain_log_injection >/dev/null 2>&1 && \
    domain_log_injection "PreToolUse" "$session_id" "gate $1 ($tool_name)"
  python3 -c '
import json, sys
print(json.dumps({"hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": sys.argv[1],
    "permissionDecisionReason": sys.argv[2],
}}))
' "$1" "$2" 2>/dev/null
  exit 0
}

# ─── (A) GIT GUARD — SIEMPRE, antes de cualquier early-exit ──────────────────
# Defensa en profundidad: aunque el flow esté activo o sea un subagente, el
# git mutante destructivo NUNCA pasa por el agente.
if [ "$tool_name" = "Bash" ]; then
  git_destructive=$(printf '%s' "$tool_cmd" | python3 -c '
import re, sys
cmd = sys.stdin.read()
pats = [
    r"git\s+reset\s+--hard",
    r"git\s+clean\b",
    r"git\s+stash\b",
    r"git\s+checkout\s+(--|\.)",
]
print("yes" if any(re.search(p, cmd) for p in pats) else "")
' 2>/dev/null)
  if [ "$git_destructive" = "yes" ]; then
    emit_decision "deny" "domain git-guard: comando git destructivo bloqueado (reset --hard / clean / stash / checkout -- | .). El agente NUNCA ejecuta git mutante sobre tu working tree. Si de verdad lo necesitas, córrelo tú manualmente fuera del agente."
  fi
fi

# ─── (C) COMMIT-GATE — antes del early-exit por flow ─────────────────────────
# git commit (no --amend) exige una corrida de tests verificada en la sesión:
# marker fresco ~/.local/state/domain/tests-ok-<session> (lo escribe el hook
# post-test). Sin marker fresco → ask (default/plan) o deny (modos automáticos).
if [ "$tool_name" = "Bash" ]; then
  is_commit=$(printf '%s' "$tool_cmd" | python3 -c '
import re, sys
cmd = sys.stdin.read()
if re.search(r"\bgit\s+commit\b", cmd) and not re.search(r"--amend", cmd):
    print("yes")
' 2>/dev/null)
  if [ "$is_commit" = "yes" ]; then
    marker="$HOME/.local/state/domain/tests-ok-$session_id"
    fresh=""
    # fresco = existente y modificado en los últimos 120 minutos.
    [ -f "$marker" ] && [ -n "$(find "$marker" -mmin -120 2>/dev/null)" ] && fresh="yes"
    if [ "$fresh" != "yes" ]; then
      case "$perm_mode" in
        default|plan) commit_dec="ask" ;;
        *)            commit_dec="deny" ;;
      esac
      emit_decision "$commit_dec" "domain commit-gate: no hay corrida de tests verificada en esta sesión (falta el marker fresco tests-ok). Corre la suite de tests antes de commitear — el hook post-test deja el marker. ¿Commit igual?"
    fi
  fi
fi

# Flow SDD activo en la sesión → pasa el resto (edición de código).
[ -r "$HOME/.local/state/domain/flow-$session_id" ] && exit 0

# ─── (B) Bash: solo gatear si el comando parece edición de código ────────────
if [ "$tool_name" = "Bash" ]; then
  is_edit=$(printf '%s' "$tool_cmd" | python3 -c '
import re, sys
cmd = sys.stdin.read()
code_ext = r"\.(go|py|ts|tsx|js|jsx|sql|sh|bash|rs|java|kt|php|rb|c|cc|cpp|h|hpp|vue|svelte)\b"
patterns = [
    r"\bsed\s+(-\w*\s+)*-i",                              # sed -i (in-place)
    r"\btee\s+(-a\s+)?\S*" + code_ext,                    # tee a archivo de código
    r"\bpatch\b",
    r"\bgit\s+apply\b",
    r">>?\s*\S*" + code_ext,                              # redirect a archivo de código
    r"\bperl\s+(-\w*\s+)*-i",                             # perl -i (in-place)
    r"\bpython3?\b[^|]*-c\b[\s\S]*open\s*\([^)]*[\x27\x22][wa]",  # python -c open(...,"w"/"a")
    r"\bdd\b[^|]*\bof=",                                  # dd of=<archivo>
    r"\b(cp|mv)\s+[\s\S]*" + code_ext,                    # cp/mv hacia (o desde) código
    r"<<[-~]?\s*[\x27\x22]?\w+[\x27\x22]?[\s\S]*" + code_ext,  # here-doc a archivo de código
]
if any(re.search(p, cmd) for p in patterns):
    print("yes")
' 2>/dev/null)
  [ "$is_edit" = "yes" ] || exit 0
fi

# Sin flow y tocando código → decidir según el modo de permisos.
case "$perm_mode" in
  default|plan) decision="ask" ;;
  *)            decision="deny" ;;
esac

reason="domain (issue-54.7): edición de código SIN flow SDD activo. TODO código pasa por SDD: ejecuta domain_orchestrate (el spec se arma en la fase sdd-spec — consulta dudas al usuario ANTES de redactarlo). Si el usuario ordenó explícitamente saltear el SDD, pídele que apruebe esta edición."

emit_decision "$decision" "$reason"
