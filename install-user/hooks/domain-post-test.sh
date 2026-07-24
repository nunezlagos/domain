#!/usr/bin/env bash
# hooks/domain-post-test.sh — hook PostToolUse de Claude Code (matcher Bash).
#
# Alimenta el commit-gate con el resultado de las corridas de tests. Cuando el
# comando ejecutado es una suite de tests (go test, npm test, pytest, jest,
# vitest, cargo test, phpunit, rspec) el hook decide:
#
#   - Tests OK (exit 0 / sin señal de fallo en el output)  → toca el marker
#     '~/.local/state/domain/tests-ok-<session>'.
#   - Tests en rojo (exit != 0 / señal de fallo en output) → BORRA el marker,
#     para que el commit-gate no deje comitear sobre tests rojos.
#
# Si el comando NO es una corrida de tests, es no-op (no toca el marker).
#
# Best-effort total: exit 0 siempre, nunca bloquea ni rompe la sesión. Ante
# cualquier fallo de parseo, sale sin tocar el marker.
set +e

payload=$(cat)

# DOMAINSERV-71: si python3 no está disponible, no podemos parsear → skip
[ -n "$payload" ] && command -v python3 >/dev/null 2>&1 || exit 0

eval "$(printf '%s' "$payload" | python3 -c '
import json, re, sys, shlex

try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)

session_id = d.get("session_id", "")
cmd = (d.get("tool_input") or {}).get("command", "") or ""

# ¿El comando es una corrida de tests? (una de las suites soportadas)
test_re = re.compile(
    r"\b(go\s+test|npm\s+(run\s+)?test|pytest|jest|vitest|cargo\s+test|phpunit|rspec)\b"
)
is_test = bool(test_re.search(cmd))

# Reunimos el texto de salida y cualquier indicador explícito de estado del
# tool_response para decidir OK/rojo.
resp = d.get("tool_response")
out_parts = []
explicit_fail = False
explicit_ok = None  # None = sin dato explícito

if isinstance(resp, list):
    # DOMAINSERV-108: algunos clientes entregan tool_response como lista
    # [{type,text}] (igual que las tools MCP). Extraemos el texto de cada item.
    for c in resp:
        if isinstance(c, dict) and isinstance(c.get("text"), str):
            out_parts.append(c["text"])
elif isinstance(resp, dict):
    for k in ("stdout", "stderr", "output", "content", "result"):
        v = resp.get(k)
        if isinstance(v, str):
            out_parts.append(v)
    # Código de salida numérico, si el runtime lo expone.
    for k in ("exit_code", "exitCode", "returncode", "code", "status"):
        v = resp.get(k)
        if isinstance(v, bool):
            continue
        if isinstance(v, int):
            explicit_ok = (v == 0)
            break
    # Flags booleanos de error / interrupción.
    for k in ("isError", "is_error"):
        if resp.get(k) is True:
            explicit_fail = True
    if resp.get("interrupted") is True:
        explicit_fail = True
    s = resp.get("success")
    if isinstance(s, bool) and explicit_ok is None:
        explicit_ok = s
elif isinstance(resp, str):
    out_parts.append(resp)

output = "\n".join(out_parts)

# Señales de fallo en el output de las suites soportadas. Diseñadas para NO
# disparar con "0 failed" / "0 failures" (exigen un número >= 1).
fail_patterns = [
    r"(?m)^FAIL\b",                          # go: línea FAIL\tpkg
    r"--- FAIL:",                            # go: subtests
    r"\bpanic:\s",                           # go: panic
    r"test result:\s*FAILED",                # cargo test
    r"FAILURES!",                            # phpunit
    r"(?im)\b[1-9]\d*\s+failed\b",           # jest/vitest/pytest: "N failed"
    r"(?im)\b[1-9]\d*\s+failing\b",          # mocha: "N failing"
    r"(?im)\b[1-9]\d*\s+failure(s)?\b",      # rspec: "N failures"
]
signal_fail = any(re.search(p, output) for p in fail_patterns)

# Resolución final: prioriza el estado explícito; si no hay, usa las señales
# del output; si tampoco hay señales, asume OK (el comando corrió).
if explicit_fail:
    ok = False
elif explicit_ok is not None:
    ok = explicit_ok and not signal_fail
else:
    # DOMAINSERV-108: el tool_response de Bash en Claude Code NO expone exit_code
    # (shape: {stdout,stderr,interrupted,isImage,noOutputExpected}). El fail-closed
    # anterior (ok=False sin estado explícito) hacía que el marker tests-ok NUNCA
    # se escribiera para `go test` → el commit-gate quedaba insatisfacible por vía
    # automática. Inferimos OK cuando el comando ES una corrida de tests, no fue
    # interrumpido (interrupted=True ya setea explicit_fail arriba) y el output no
    # muestra señales de fallo. Un fallo real dispara fail_patterns (FAIL/panic/…).
    ok = is_test and not signal_fail

print("session_id=%s" % shlex.quote(session_id))
print("is_test=%s" % shlex.quote("1" if is_test else "0"))
print("tests_ok=%s" % shlex.quote("1" if ok else "0"))
' 2>/dev/null)"

# Sin session_id o comando que no es test → no-op.
[ -n "$session_id" ] || exit 0
[ "$is_test" = "1" ] || exit 0

state_dir="$HOME/.local/state/domain"
mkdir -p "$state_dir" 2>/dev/null
marker="$state_dir/tests-ok-$session_id"

if [ "$tests_ok" = "1" ]; then
  # DOMAINSERV-74: marker con timestamp + tree hash (git diff HEAD) para
  # invalidar ante cualquier edición posterior.
  tree_hash=$(git diff --no-color HEAD 2>/dev/null | sha256sum 2>/dev/null | cut -d' ' -f1)
  printf '%s\t%s\n' "$(date -Iseconds)" "${tree_hash:-}" > "$marker" 2>/dev/null
else
  rm -f "$marker" 2>/dev/null
fi
exit 0
