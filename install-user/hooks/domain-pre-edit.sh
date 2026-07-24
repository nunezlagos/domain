#!/usr/bin/env bash
# hooks/domain-pre-edit.sh — hook PreToolUse de Claude Code (Edit|Write|
# NotebookEdit|Bash).
#
# REQ-54 issue-54.7: gate determinista SDD-para-código. TODO código pasa por
# SDD (decisión del usuario, sin exención trivial):
#
#   - Si la sesión tiene flow SDD activo validado por token HMAC server-side
#     (domain-post-orchestrate.sh genera el token vía domain_flow_grant_token)
#     → la edición pasa.
#   - Sin flow o token inválido: en modo normal (default/plan) → permissionDecision
#     "ask" (el HUMANO decide en el diálogo); en modos automáticos (acceptEdits/
#     bypassPermissions/auto) → "deny" con razón: el agente es FORZADO a
#     orquestar primero.
#   - Bash solo se gatea si el comando PARECE edición de código (sed -i, tee,
#     patch, git apply, redirect, perl -i, python -c open(w), cp/mv, dd of=,
#     here-doc a archivo de código). Limitación conocida: heurística con
#     falsos negativos posibles — la policy explícita cubre el resto.
#
# Endurecimiento (pre-edit-hardening):
#   (A) GIT GUARD: git destructivo (reset --hard / clean / stash mutante /
#       checkout -- | . / restore / rm / worktree remove) → deny SIEMPRE,
#       incluso con flow activo o en subagentes. Normaliza global options
#       (-C, -c, --git-dir, --work-tree) para evitar evasiones. Defensa en
#       profundidad por si el permissions.deny fallara. `git stash list|show`
#       es read-only y pasa (DOMAINSERV-111).
#   (B) heurística de edición ampliada (ver arriba) para atrapar bypass.
#   (C) COMMIT-GATE: git commit sin marker fresco de tests verificados →
#       ask (default/plan) o deny (modos automáticos).
#   (D) SCOPE POR EXTENSIÓN (DOMAINSERV-111): Edit/Write/NotebookEdit sobre un
#       archivo que NO es código (.md, .txt, .log, .csv, scratchpad) pasa sin
#       gate. El SDD gobierna código; antes esta rama ignoraba file_path y
#       bloqueaba notas y docs, mientras el mismo archivo vía Bash heredoc
#       pasaba. Ambas ramas comparten ahora DOMAIN_CODE_EXTS.
#
# Best-effort en fallos de parseo: permitir (exit 0) antes que romper la sesión.
set +e

# Lib compartida (best-effort): aporta domain_log_injection. Si no está,
# el hook igual funciona — el logging es opcional, jamás bloquea.
LIB="$(dirname "$0")/domain-hooks-lib.sh"
[ -r "$LIB" ] && . "$LIB"

# DOMAINSERV-111: fuente ÚNICA de qué extensión cuenta como código. La consumen
# la rama Bash (heurística sobre el comando) y la rama Edit/Write (file_path).
# Estaban divergidas: Bash filtraba por extensión y Edit/Write no, así que un
# .md se bloqueaba por Write pero pasaba por heredoc.
export DOMAIN_CODE_EXTS='go|py|ts|tsx|js|jsx|sql|sh|bash|rs|java|kt|php|rb|c|cc|cpp|h|hpp|vue|svelte|yaml|yml|json|toml|tf|hcl|env|xml|gradle|cs|scala|swift|proto|lua'

payload=$(cat)

# DOMAINSERV-71: fail-closed — el gate necesita python3 para parsear el payload
if [ -n "$payload" ] && ! command -v python3 >/dev/null 2>&1; then
  echo '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"CRITICAL (DOMAINSERV-71): python3 no está disponible. El gate SDD no puede operar sin python3. Instala python3 y reintenta."}}'
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
print("session_id=%s" % shlex.quote(d.get("session_id", "")))
print("tool_name=%s" % shlex.quote(d.get("tool_name", "")))
print("perm_mode=%s" % shlex.quote(d.get("permission_mode", "default")))
print("tool_cmd=%s" % shlex.quote(ti.get("command", "") if isinstance(ti, dict) else ""))
print("file_path=%s" % shlex.quote(ti.get("file_path", "") if isinstance(ti, dict) else ""))
' 2>/dev/null)"

# DOMAINSERV-103: payload no-vacío pero no parseable como JSON → fail-closed.
# Mismo criterio que python3-ausente (DOMAINSERV-71): no podemos derivar perm_mode
# de un payload corrupto, así que denegamos en vez de exit 0 (que dejaba pasar la
# edición sin gate, antes del git-guard y del commit-gate).
if [ -n "$payload" ] && [ "${parse_failed:-}" = "yes" ]; then
  echo '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"CRITICAL (DOMAINSERV-103): el payload del hook no es JSON parseable. El gate SDD no puede operar sobre un payload corrupto, así que fail-closed (deny)."}}'
  exit 0
fi
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

# DOMAINSERV-111: el cuerpo de un here-doc y el texto entrecomillado son DATOS
# (mensaje de commit, documentación), no comandos — mencionar "git reset --hard"
# no lo ejecuta. Sin esto, un commit que DOCUMENTA el guard se auto-bloqueaba.
# Excepción fail-closed: si hay un intérprete que EJECUTA el literal, no se
# strippea nada. El reemplazo es un token SIN espacios a propósito: vaciarlo
# rompería el normalizador de opciones globales de abajo
# (git -C "/p" reset --hard → git  reset --hard → git --hard, un bypass).
if not re.search(r"\b(?:bash|sh|zsh|dash|ksh)\s+(?:-\w+\s+)*-c\b|\beval\b|\bxargs\b", cmd):
    cmd = re.sub(r"<<-?\s*([\x27\x22]?)(\w+)\1[\s\S]*?^\2$", " LITERAL ", cmd, flags=re.M)
    cmd = re.sub(r"\x27[^\x27]*\x27", " LITERAL ", cmd)
    cmd = re.sub(r"\x22[^\x22]*\x22", " LITERAL ", cmd)

# strip git global options between "git" and subcommand to prevent
# evasion via git -C . reset --hard, git -c x=y stash, etc.
normalized = re.sub(
    r"\bgit\s+(?:-[cC]\s+\S+\s+|--(?:git-dir|work-tree)(?:=\S+|\s+\S+)?\s+)*",
    "git ",
    cmd
)
pats = [
    r"git\s+reset\s+--hard",
    r"git\s+clean\b",
    # DOMAINSERV-111: list/show son READ-ONLY, no mutan el working tree
    r"git\s+stash\b(?!\s+(?:list|show)\b)",
    r"git\s+checkout\s+(--|\.)",
    r"git\s+restore\b",
    r"git\s+rm\b",
    r"git\s+worktree\s+remove\b",
]
print("yes" if any(re.search(p, normalized) for p in pats) else "")
' 2>/dev/null)
  if [ "$git_destructive" = "yes" ]; then
    emit_decision "deny" "domain git-guard: comando git destructivo bloqueado (reset --hard / clean / stash / checkout -- | . / restore / rm / worktree remove). El agente NUNCA ejecuta git mutante sobre tu working tree. Si de verdad lo necesitas, córrelo tú manualmente fuera del agente."
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
if re.search(r"\bgit\s+commit\b", cmd):
    print("yes")
' 2>/dev/null)
  if [ "$is_commit" = "yes" ]; then
    # DOMAINSERV-108: flows MICRO (ediciones triviales sin lógica testeable:
    # texto de front, script nuevo, doc/config) están EXENTOS del requisito de
    # tests (decisión explícita del usuario). El post-orchestrate escribe el
    # modo del flow como field3 del marker; si es "micro" el commit pasa sin
    # tests-ok. Cualquier otro modo mantiene el gate DOMAINSERV-74 intacto.
    flow_marker="$HOME/.local/state/domain/flow-$session_id"
    flow_mode=""
    [ -r "$flow_marker" ] && flow_mode=$(head -1 "$flow_marker" 2>/dev/null | cut -f3)
    marker="$HOME/.local/state/domain/tests-ok-$session_id"
    fresh=""
    if [ "$flow_mode" = "micro" ]; then
      fresh="yes"
    fi
    # DOMAINSERV-74: marker debe existir, tener < 30 min, y el tree hash
    # debe coincidir con el working tree actual (invalida ante ediciones
    # posteriores a la corrida de tests).
    if [ "$fresh" != "yes" ] && [ -f "$marker" ] && [ -n "$(find "$marker" -mmin -30 2>/dev/null)" ]; then
      stored_hash=$(cut -f2 "$marker" 2>/dev/null)
      # DOMAINSERV-95: sin hash almacenado → NO fresco (fail-closed). Un marker
      # legacy (solo-timestamp) o forjado con printf ya no habilita el commit.
      if [ -n "$stored_hash" ]; then
        current_hash=$(git diff --no-color HEAD 2>/dev/null | sha256sum 2>/dev/null | cut -d' ' -f1)
        [ "$current_hash" = "$stored_hash" ] && fresh="yes"
      fi
    fi
    if [ "$fresh" != "yes" ]; then
      case "$perm_mode" in
        default|plan) commit_dec="ask" ;;
        *)            commit_dec="deny" ;;
      esac
      emit_decision "$commit_dec" "domain commit-gate (DOMAINSERV-74): no hay corrida de tests que cubra el estado actual del código. El marker tests-ok falta, expiró (30 min) o el working tree cambió después de los tests. Corre la suite de tests antes de commitear. ¿Commit igual?"
    fi
  fi
fi

# ─── Flow SDD activo: validación server-side (FAIL-CLOSED, DOMAINSERV-70) ─────
#    v2 token: marker = HMAC token + expires_at. SOLO un token VÁLIDO contra el
#    server (firma + flow activo) habilita la edición.
#    v1 legacy: timestamp + flow_run_id, validado contra flow_status real.
#    Si el server NO puede validar (token inválido / server unreachable / sin
#    creds / sin lib) NO se confía en el marker local — es forjable por el propio
#    agente (mismo uid) — y se cae al gate (ask en modo normal, deny en modos
#    automáticos). Se removió todo trust-local degradado por TTL.

marker="$HOME/.local/state/domain/flow-$session_id"
if [ -r "$marker" ] && [ -r "$LIB" ]; then
  . "$LIB"
  domain_resolve_env 2>/dev/null || true

  first_line=$(head -1 "$marker" 2>/dev/null)
  field1=$(printf '%s' "$first_line" | cut -f1)
  field2=$(printf '%s' "$first_line" | cut -f2)

  # sin vps_url/api_key no hay forma de validar contra el server → gate
  if [ -n "$vps_url" ] && [ -n "$api_key" ]; then
    domain_mcp_init >/dev/null 2>&1
    # DOMAINSERV-107: enrutar por field2 (UUID del flow_run), NO por un glob
    # sobre field1. El marker legacy es "<timestamp>\t<flow_run_id>": su field1
    # (timestamp) matcheaba el glob de token v2 [A-Za-z0-9_-]*... y enrutaba a
    # validate_token, dejando la rama legacy (flow_status) INALCANZABLE → gate
    # insatisfacible en servers sin HMAC secret. El marker v2 es
    # "<token>\t<expires_at>": field2 = timestamp ISO, nunca un UUID.
    if printf '%s' "$field2" | grep -qE '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'; then
      # v1 legacy: field2 = flow_run_id → validar vía flow_status (running/pending)
      resp=$(domain_call_tool domain_flow_status \
        "{\"flow_run_id\":\"$field2\"}" 2>&1)
      # DOMAINSERV-108: flow_status devuelve JSON indentado ("status": "pending"
      # con espacio tras el colon). El patrón sin espacio nunca matcheaba →
      # rama legacy muerta. Tolerar whitespace opcional tras el colon.
      echo "$resp" | grep -qE '"status":[[:space:]]*"(running|pending)"' && exit 0
      # no confirmado / server unreachable → fail-closed → gate
    elif [ -n "$field1" ]; then
      # v2: field1 = token HMAC → validar firma + flow activo server-side
      resp=$(domain_call_tool domain_flow_validate_token \
        "{\"token\":\"$field1\",\"session_id\":\"$session_id\"}" 2>/dev/null)
      # vinfo = "<valid>\t<allowed_paths_json>" (DOMAINSERV-110 batch-mode)
      vinfo=$(printf '%s' "$resp" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
    for c in d.get("result",{}).get("content",[]):
        if isinstance(c, dict) and c.get("type") == "text":
            body = json.loads(c["text"])
            if body.get("valid"):
                ap = body.get("allowed_paths") or []
                print("yes\t" + json.dumps(ap))
            break
except Exception:
    pass
' 2>/dev/null)
      valid=$(printf '%s' "$vinfo" | cut -f1)
      allowed_json=$(printf '%s' "$vinfo" | cut -f2)
      if [ "$valid" = "yes" ]; then
        # sin allowlist (flow normal) → sin restricción de path (backward-compat).
        if [ -z "$allowed_json" ] || [ "$allowed_json" = "[]" ] || [ "$allowed_json" = "null" ]; then
          exit 0
        fi
        # sin file_path (ej. Bash-edit) no podemos scopear por path: el token es
        # válido, dejamos pasar; la allowlist aplica a Edit/Write/NotebookEdit.
        [ -z "$file_path" ] && exit 0
        # el flow declaró paths: la edición pasa solo si el path matchea un glob.
        match=$(FP="$file_path" AJ="$allowed_json" python3 -c '
import os, sys, json, fnmatch
fp = os.environ.get("FP","")
try:
    globs = json.loads(os.environ.get("AJ","[]"))
except Exception:
    globs = []
cands = {fp}
cwd = os.getcwd()
if fp.startswith(cwd + "/"):
    cands.add(fp[len(cwd)+1:])
for g in globs:
    for c in cands:
        if fnmatch.fnmatch(c, g):
            print("yes"); sys.exit(0)
' 2>/dev/null)
        [ "$match" = "yes" ] && exit 0
        emit_decision "deny" "domain batch-mode (DOMAINSERV-110): el path '$file_path' está fuera de la allowlist del flow activo (paths permitidos: $allowed_json). Este flow scopea las ediciones a sus paths declarados — editá dentro del scope o abrí un flow para este path."
      fi
      # inválido o server unreachable → fail-closed → cae al gate
    fi
  fi
fi
# marker ausente, no validable o inválido → cae al gate de abajo (ask/deny)

# ─── (B) Bash: solo gatear si el comando parece edición de código ────────────
if [ "$tool_name" = "Bash" ]; then
  is_edit=$(printf '%s' "$tool_cmd" | python3 -c '
import os, re, sys
cmd = sys.stdin.read()

code_ext = r"\.(" + os.environ["DOMAIN_CODE_EXTS"] + r")\b"
# DOMAINSERV-96: el redirect bare `>` omite json|yaml|yml|xml — son destino
# frecuente de VOLCADO de output (curl > x.json, kubectl get -o yaml > x.yaml),
# no edición de fuente. Esos formatos siguen gateados vía tee/cp/heredoc.
src_ext = r"\.(go|py|ts|tsx|js|jsx|sql|sh|bash|rs|java|kt|php|rb|c|cc|cpp|h|hpp|vue|svelte|toml|tf|hcl|env|gradle|cs|scala|swift|proto|lua)\b"
# separadores de comando: los patrones de verbo no deben cruzarlos (evita
# que un token de código en otro subcomando dispare el gate).
sep = r"[^&;|]*"
# ancla de comando: inicio de línea, tras separador, o tras sudo.
cmd0 = r"(?:^|[;&|]\s*|\bsudo\s+)"

# DOMAINSERV-75: patrones de escritura a archivos de código. Cualquier
# coincidencia → el comando parece editar código y requiere gate SDD.
patterns = [
    # Editores in-place
    r"\bsed\s+(-\w*\s+)*-i",
    r"\bperl\s+(-\w*\s+)*-i",
    r"\bawk\b[^|]*\b-i\s+inplace\b",
    # shell a archivos
    r"\btee\s+(-a\s+)?\S*" + code_ext,
    r">>?\s*\S*" + src_ext,
    r"\bdd\b[^|]*\bof=",
    r"\btruncate\s+-s\b",
    r"\b(cp|mv)\s+" + sep + code_ext,
    # parches / apply
    cmd0 + r"patch\s",
    r"\bgit\s+apply\b",
    # here-doc a archivos de código
    r"<<[-~]?\s*[\x27\x22]?\w+[\x27\x22]?[\s\S]*" + code_ext,
    # escritura con intérpretes en línea
    r"\bpython3?\b[^|]*-c\b[\s\S]*(?:open\s*\([^)]*[\x27\x22][wa]|write_text|writelines)",  # python -c open/write_text/writelines
    r"\bnode\b[^|]*-(?:e|eval)\b[\s\S]*(?:writeFileSync|appendFileSync|writeSync|openSync\s*\([^)]*[\x27\x22][wa])",  # node -e write
    r"\bruby\b[^|]*-(?:e|execute)\b[\s\S]*(?:File\.\s*(?:write|open)|IO\.\s*(?:write|open))",  # ruby -e write
    # editores de línea
    r"\b(?:ed|ex)\s+\S+",
    # install (copia con permisos) — anclado para no matchear pip/apt install
    cmd0 + r"install\s+-[^-]*[moc]",
    # archivos sin extensión canónica
    r"\b(?:cp|mv|tee)\s+(-a\s+)?\S*(?:Dockerfile|Makefile)\b",
    r">>?\s*\S*(?:Dockerfile|Makefile)\b",
]
if any(re.search(p, cmd) for p in patterns):
    print("yes")
' 2>/dev/null)
  [ "$is_edit" = "yes" ] || exit 0
fi

# ─── (D) Edit/Write/NotebookEdit: gatear SOLO archivos de código ─────────────
# DOMAINSERV-111: esta rama no miraba file_path, así que el gate frenaba
# escribir una nota .md, un .txt de análisis o el scratchpad — operaciones que
# no son código y que el SDD no gobierna. Peor: el MISMO archivo escrito con
# Bash heredoc sí pasaba, porque allá la heurística sí filtra por extensión.
# Ahora ambas ramas comparten DOMAIN_CODE_EXTS y el criterio es uno solo.
if [ "$tool_name" != "Bash" ] && [ -n "$file_path" ]; then
  is_code=$(printf '%s' "$file_path" | python3 -c '
import os, re, sys
fp = sys.stdin.read().strip()
code_ext = r"\.(" + os.environ["DOMAIN_CODE_EXTS"] + r")$"
# Dockerfile/Makefile son código sin extensión canónica (mismo criterio que Bash)
noext = r"(?:^|/)(?:Dockerfile|Makefile)[^/]*$"
if re.search(code_ext, fp) or re.search(noext, fp):
    print("yes")
' 2>/dev/null)
  [ "$is_code" = "yes" ] || exit 0
fi

# Sin flow y tocando código → decidir según el modo de permisos.
case "$perm_mode" in
  default|plan) decision="ask" ;;
  *)            decision="deny" ;;
esac

reason="domain (issue-54.7): edición de código SIN flow SDD activo. TODO código pasa por SDD. Vía rápida: ejecuta domain_orchestrate (mode express para cambios ≤10 líneas single-file, lite para cambios chicos) — eso arma el flow y habilita la edición; en la fase sdd-spec consulta dudas al usuario ANTES de redactar. Nota: este gate es determinista y NO puede recibir una aprobación por chat; la única vía es abrir el flow."

emit_decision "$decision" "$reason"
