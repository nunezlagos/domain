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
#   (A) GIT GUARD: clasifica por RECUPERABILIDAD (DOMAINSERV-278), no por
#       "es mutante". deny para lo irrecuperable (reset --hard / clean /
#       stash drop|clear / worktree remove --force); ask para lo que git puede
#       revertir (stash / checkout -- | . / restore / rm / worktree remove), que
#       aprueba el humano en el diálogo. Ambos corren SIEMPRE, incluso con flow
#       activo o en subagentes. Normaliza global options (-C, -c, --git-dir,
#       --work-tree) para evitar evasiones. Defensa en profundidad por si el
#       permissions.deny/ask fallara. `git stash list|show` es read-only y pasa
#       (DOMAINSERV-111).
#   (A2) GUARD DESTRUCTIVO (DOMAINSERV-222): lo que el git-guard NO cubre porque
#       no se recupera con git. NO mira el FLAG, mira el RADIO DE DAÑO: rm
#       RECURSIVO del cwd / raíz del repo / un ancestro / . / $HOME / .git / una
#       raíz de app de container (/app /srv /repo /workspace /var/www), rm de
#       secretos NO trackeados en git (con o sin flags), SQL destructivo, y la
#       escritura del propio marker de bypass. Un `rm -f archivo` común NO entra
#       (medía 79% de los disparos sobre 21.105 comandos reales). Atraviesa
#       envolturas (docker exec/run, kubectl exec, ssh, scp, xargs, find,
#       sh -c/eval, psql, mysql). ask en todos los modos salvo
#       bypassPermissions, que es deny duro, con bypass de un solo uso que
#       escribe el HUMANO. Alcance completo y HUECOS CONOCIDOS (lista real):
#       hook_destructive_guard_test.go.
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
# DOMAINSERV-218: presente solo dentro de un subagente. Ausente = hilo principal.
print("agent_id=%s" % shlex.quote(d.get("agent_id", "") or ""))
print("tool_name=%s" % shlex.quote(d.get("tool_name", "")))
print("perm_mode=%s" % shlex.quote(d.get("permission_mode", "default")))
print("tool_cmd=%s" % shlex.quote(ti.get("command", "") if isinstance(ti, dict) else ""))
print("file_path=%s" % shlex.quote(ti.get("file_path", "") if isinstance(ti, dict) else ""))
# DOMAINSERV-222 (radio de daño): el guard destructivo necesita saber DÓNDE está
# parada la sesión para decidir si el objetivo de un rm -rf es el proyecto entero.
print("payload_cwd=%s" % shlex.quote(d.get("cwd", "")))
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

# domain_flow_marker_de_este_agente — el marker de flow que autoriza a QUIEN está editando.
#
# DOMAINSERV-218: hasta acá había UN marker por sesión, así que N subagentes de un mismo flow
# compartían una sola allowlist y era imposible denegarle a uno un path que le correspondía a
# otro (criterio #2 del ticket). Un subagente tiene su propio marker porque agent_id SÍ lo
# distingue —el session_id no, se hereda del padre—.
#
# EL FALLBACK AL MARKER DE SESIÓN NO ES PEREZA, es lo que mantiene el gate satisfacible: un
# subagente al que el orquestador todavía no le emitió un token propio seguiría necesitando
# editar bajo el flow que abrió el padre. Sin el fallback quedaría bloqueado, y un gate que
# deniega lo legítimo empuja al bypass permanente — el modo de falla de DOMAINSERV-111/175/195.
#
# Para el hilo principal (agent_id ausente) devuelve exactamente el mismo path que antes de este
# cambio: el camino que autoriza mis propias ediciones no se toca.
domain_flow_marker_de_este_agente() {
  base="$HOME/.local/state/domain/flow-$session_id"
  if [ -n "${agent_id:-}" ] && [ -r "${base}-${agent_id}" ]; then
    printf '%s' "${base}-${agent_id}"
    return 0
  fi
  printf '%s' "$base"
}

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
    # DOMAINSERV-222: el terminador puede venir INDENTADO (<<-EOF con tabs). Con ^\2$
    # el heredoc quedaba sin cerrar y el strip no aplicaba: mismo hueco que en (A2).
    cmd = re.sub(r"<<[-~]?\s*([\x27\x22]?)(\w+)\1[\s\S]*?^[ \t]*\2[ \t]*$", " LITERAL ", cmd, flags=re.M)
    cmd = re.sub(r"\x27[^\x27]*\x27", " LITERAL ", cmd)
    cmd = re.sub(r"\x22[^\x22]*\x22", " LITERAL ", cmd)

# strip git global options between "git" and subcommand to prevent
# evasion via git -C . reset --hard, git -c x=y stash, etc.
normalized = re.sub(
    r"\bgit\s+(?:-[cC]\s+\S+\s+|--(?:git-dir|work-tree)(?:=\S+|\s+\S+)?\s+)*",
    "git ",
    cmd
)
# DOMAINSERV-278: la clasificación es por RECUPERABILIDAD, no por "es mutante". Lo que git
# puede revertir sale a "ask" y lo decide el humano en el diálogo; deny queda para lo que no
# tiene vuelta. El criterio anterior ("todo git mutante → deny SIEMPRE") medía la categoría
# del comando en vez del daño, y eso dejaba sin puerta casos de riesgo cero: un
# `git worktree remove` sobre un worktree limpio no se podía ejecutar NI aprobar, porque el
# deny se evalúa antes que el sistema de permisos.
pats_deny = [
    r"git\s+reset\s+--hard",
    # DOMAINSERV-247: -n y --dry-run son READ-ONLY. El patrón amplio los bloqueaba, y eso
    # se midió en vivo: este mismo guard rechazó un comando de diagnóstico que solo
    # mencionaba "git clean -n" dentro de un script. Es el mismo arreglo que DOMAINSERV-111
    # hizo para stash, que había quedado a medias.
    r"git\s+clean\s+(?!.*(?:-n\b|--dry-run\b))\S",
    # drop y clear DESTRUYEN el stash: recuperarlo exige pescar dangling commits del reflog.
    # El resto de stash es recuperable y baja a pats_ask.
    r"git\s+stash\s+(?:drop|clear)\b",
    # --force borra un worktree CON cambios sin commitear. Sin el flag git se niega solo si
    # está sucio, así que ese caso es aprobable y baja a pats_ask. El flag puede venir en
    # cualquier posición (`worktree remove x --force`), por eso el lookahead y no un prefijo:
    # el prefix-match de permissions.deny no captura la segunda posición, el regex sí.
    r"git\s+worktree\s+remove\b(?=.*(?:--force\b|-f\b))",
]
# Recuperable con git: el humano aprueba en el diálogo en vez de quedarse sin camino.
pats_ask = [
    # DOMAINSERV-111: list/show son READ-ONLY, no mutan el working tree
    r"git\s+stash\b(?!\s+(?:list|show)\b)",
    r"git\s+checkout\s+(--|\.)",
    r"git\s+restore\b",
    r"git\s+rm\b",
    r"git\s+worktree\s+remove\b",
]
if any(re.search(p, normalized) for p in pats_deny):
    print("deny")
elif any(re.search(p, normalized) for p in pats_ask):
    print("ask")
else:
    print("")
' 2>/dev/null)
  if [ "$git_destructive" = "deny" ]; then
    emit_decision "deny" "domain git-guard: comando git irrecuperable bloqueado (reset --hard / clean / stash drop|clear / worktree remove --force). Esto no se revierte con git, así que el agente no lo ejecuta. Si de verdad lo necesitas, córrelo tú manualmente fuera del agente."
  elif [ "$git_destructive" = "ask" ]; then
    emit_decision "ask" "domain git-guard: comando git mutante pero RECUPERABLE (stash / checkout -- | . / restore / rm / worktree remove). Revisá el comando antes de aprobar: git puede revertirlo, pero igual toca tu working tree."
  fi
fi

# ─── (A2) GUARD DESTRUCTIVO (DOMAINSERV-222) — antes de cualquier early-exit ──
# Hermano del git-guard: el git-guard cubre lo que se recupera con git, este cubre
# lo que NO. El alcance NO mira el FLAG, mira el RADIO DE DAÑO del objetivo:
#   - radio: rm RECURSIVO cuyo objetivo resuelve al cwd, a la raíz del repo, a un
#     ancestro de cualquiera de los dos, a /, a $HOME, al .git, o a una raíz de app
#     de container (/app, /srv, /repo, /workspace, /var/www, /usr/src/app);
#   - sensible: rm de un secreto NO trackeado en git (.env*, *.key, *.pem, id_rsa,
#     *credential*, *secret*, *.p12) — el incidente original fue `rm -f .env.qa`;
#   - sql / sql-opaco: SQL destructivo, o un cliente SQL que ejecuta un archivo que
#     el guard no puede leer;
#   - automarker: el propio comando escribe el marker de bypass de este guard.
# Un `rm -f archivo` común NO entra: medía 265 de 334 disparos sobre 21.105 comandos
# reales (79%, dominado por `rm -f $SOCK`) y ninguno era la clase del incidente.
# Todo esto atravesando envolturas (docker exec/run, docker-compose, podman, kubectl
# exec, ssh, scp, xargs, find, sh -c, y eval CON y SIN comillas), porque el incidente
# llegó envuelto.
#
# EL PRINCIPIO (invertido en la tercera ronda): si el objetivo de un rm recursivo no es
# un literal que se pueda resolver con CERTEZA, se ESCALA — no se adivina. Antes el guard
# resolvía lo que podía y, cuando no podía, concluía "no es catastrófico": fail-OPEN. Esa
# era la causa ÚNICA de tres evasiones que parecían distintas (`rm -rf $PWD # PWD=tmp`,
# `F=-rf; rm $F /`, `rm -rf .g*`). Costo medido del cambio: CERO disparos nuevos sobre
# 11.358 comandos Bash únicos de 980 transcripts reales (37 → 35).
# Alcance completo y HUECOS CONOCIDOS: hook_destructive_guard_test.go.
if [ "$tool_name" = "Bash" ]; then
  # cwd del payload (Claude Code lo manda) → dónde está parada la sesión. Sin él, el
  # cwd del proceso del hook. La raíz del repo se resuelve una sola vez, read-only.
  guard_cwd="${payload_cwd:-$PWD}"
  guard_git_root=$(git -C "$guard_cwd" rev-parse --show-toplevel 2>/dev/null)
  destructivo=$(printf '%s' "$tool_cmd" | DOMAIN_GUARD_CWD="$guard_cwd" DOMAIN_GUARD_GIT_ROOT="$guard_git_root" python3 -c '
import os, re, subprocess, sys

# ─── alcance ──────────────────────────────────────────────────────────────────
SHELLS = ("sh", "bash", "zsh", "dash", "ksh")
SQL_CLIENTES = ("psql", "mysql", "mariadb", "mysqlsh", "sqlite3")
# Envolturas REMOTAS: el cwd y la raiz del repo del otro lado son DESCONOCIDOS, asi
# que ahi el radio se evalua fail-closed (ver radio()). docker/kubectl solo ejecutan
# un comando ajeno con estos subcomandos: docker rm ctr borra un container y no
# despierta el guard.
# TERCERA RONDA: el match era base(toks[0]) EXACTO contra "docker", asi que
# docker-compose (v1, con guion) y podman quedaban afuera y eran evasion directa.
REMOTAS_SUB = {"docker": ("exec", "run"), "docker-compose": ("exec", "run"),
               "podman": ("exec", "run"), "podman-compose": ("exec", "run"),
               "nerdctl": ("exec", "run"), "lxc": ("exec",), "incus": ("exec",),
               "kubectl": ("exec",), "oc": ("exec",)}
REMOTAS = ("ssh", "scp")
# Envolturas LOCALES: el objetivo se resuelve contra el cwd real.
# TERCERA RONDA: eval SIN comillas era una evasion total. `eval "rm -rf ."` disparaba porque
# el comando viajaba en un literal y la recursion lo miraba adentro; `eval rm -rf .` no tiene
# literal ninguno, asi que toks[0] quedaba en "eval", posiciones_rm no encontraba el rm en la
# posicion 0 y NADA se evaluaba — apagaba radio, sensible y sql de un solo golpe. eval es una
# envoltura LOCAL: lo que sigue corre en ESTE shell, con ESTE cwd.
LOCALES = ("xargs", "find", "eval")
WRAPPERS = ("sudo", "doas", "command", "env", "time", "timeout", "nohup", "setsid",
            "stdbuf", "exec", "ionice", "nice")
# flags de wrapper que CONSUMEN el token siguiente (sudo -u www-data, nice -n 10,
# timeout -k 5). Sin esto el wrapper se pelaba pero su valor quedaba como toks[0].
WRAP_VALOR = ("-u", "-g", "-n", "-o", "-k", "-s", "-p", "-C", "--user", "--group",
              "--signal", "--kill-after", "--adjustment", "--class", "--classdata",
              "--chdir", "--output")
# gramatica de shell: estos tokens NO son el comando del segmento.
DESCARTES = ("then", "do", "else", "elif", "if", "while", "until", "{", "(", "!",
             "&&", "||", "|&")
# Raices de app en containers: /app suele ser un BIND MOUNT del repo del host y no lo
# parece. Este es el caso que mordio al usuario.
RAICES_APP = ("/app", "/srv", "/repo", "/workspace", "/var/www", "/usr/src/app", "/code")
EFIMEROS = ("/tmp", "/var/tmp", "/dev/shm", "/var/cache", "/var/log", "/run")
# Subdirectorios que se regeneran: borrarlos recursivamente es rutina de desarrollo.
RUTINA = ("node_modules", "dist", "build", "vendor", "target", ".next", ".nuxt", "out",
          "__pycache__", ".pytest_cache", ".mypy_cache", ".venv", "venv", ".cache",
          "tmp", "temp", "logs", "bin", "obj", ".terraform", ".gradle")
# Archivos que NO viven en git: borrarlos no se revierte con un checkout.
SENSIBLES = (r"^\.env(?:$|[.\-_])", r"^id_(?:rsa|dsa|ecdsa|ed25519)(?:$|\.)",
             r"\.(?:key|pem|p12|pfx|jks)(?:$|\.)", r"credential", r"secret")
# .env.example / secrets.md son PLANTILLA y DOC: van al repo, no son el secreto.
EJEMPLAR = r"\.(?:example|sample|template|dist|tpl|md|rst)$"
# mkdir es el peor de la lista: crea el marker como DIRECTORIO, y el rm -f con el que el
# gate lo consume no borra directorios, asi que el bypass quedaba PERMANENTE
VERBOS_ESCRITURA = ("tee", "cp", "mv", "touch", "ln", "install", "dd", "rsync", "mkdir")
# Interpretes que pueden ESCRIBIR el marker desde su propio literal (open(...,w)).
INTERPRETES_ESCRITURA = ("python", "python2", "python3", "node", "ruby", "perl", "php")
# TERCERA RONDA — el mapa de asignaciones NO puede sobrescribir las variables con las que
# el guard MIDE el radio. Envenenarlas era apagar el guard con su propia herramienta:
# `rm -rf $PWD # PWD=tmp` resolvia a ./tmp y pasaba.
PROTEGIDAS = ("PWD", "OLDPWD", "HOME", "CWD", "PROJECT", "PROJECT_ROOT", "REPO",
              "REPO_ROOT", "GIT_ROOT", "ROOT", "WORKSPACE")
# Metacaracteres de expansion de PATH. bash los resuelve; el guard los comparaba como
# texto plano, asi que `.g*` (= .git .github .gitignore) le parecia un nombre literal.
GLOB = "*?[]{}"


def norm(p):
    return os.path.normpath(p) if p else ""


def ctx_base():
    cwd = os.environ.get("DOMAIN_GUARD_CWD") or os.getcwd()
    raiz = os.environ.get("DOMAIN_GUARD_GIT_ROOT") or ""
    home = os.environ.get("HOME") or ""
    return {"cwd": norm(cwd), "raiz": norm(raiz), "home": norm(home), "remoto": False}


def base(tok):
    return tok.rsplit("/", 1)[-1]


def expandir(tok, lits):
    def uno(m):
        i = int(m.group(1))
        return lits[i] if i < len(lits) else m.group(0)
    return re.sub("\x01(\\d+)\x01", uno, tok)


def limpiar(tok, lits):
    # CAMBIO 2.2: expandir ANTES de base(). Sin esto el token de comando podia venir
    # entrecomillado o con escape (rm entre comillas, \rm, /bin/rm entrecomillado) y
    # base() miraba un placeholder.
    t = expandir(tok, lits)
    t = re.sub(r"^\\+", "", t)
    t = t.strip("\x22\x27")
    # TERCERA RONDA: el cierre del subshell viaja PEGADO al ultimo token — (rm -rf .)
    # dejaba el objetivo como ".)" y radio() no lo reconocia. Se saca solo el parentesis
    # que NO abrio este token, asi $(pwd) y $(mktemp -d) quedan intactos.
    while t.endswith(")") and t.count("(") < t.count(")"):
        t = t[:-1]
    return t


def enmascarar(texto, lits):
    # El cuerpo de un heredoc y lo entrecomillado son DATOS, no comandos. No se borran:
    # se reemplazan por un placeholder INDEXADO, asi el comando que si ejecuta su
    # literal (sh -c, psql -c, ssh) lo recupera y mira adentro.
    def guardar(m):
        lits.append(m.group(m.re.groups))
        return "\x01%d\x01" % (len(lits) - 1)
    # CAMBIO 2.3: el terminador puede venir INDENTADO (<<-SQL con tabs). Con ^\1$ el
    # heredoc quedaba sin cerrar: evasion y a la vez falso positivo al documentarlo.
    texto = re.sub(r"<<[-~]?\s*[\x27\x22]?(\w+)[\x27\x22]?([\s\S]*?)^[ \t]*\1[ \t]*$",
                   guardar, texto, flags=re.M)
    return enmascarar_comillas(texto, lits)


def enmascarar_comillas(texto, lits):
    # TERCERA RONDA. Antes eran DOS regex independientes (todas las comillas simples,
    # despues todas las dobles) y eso rompia por los dos lados:
    #   - `echo "it\x27s" && rm -rf . && echo "that\x27s"`: los dos apostrofos de adentro
    #     pareaban entre si y se comian el rm del medio. El comando pasaba entero.
    #   - `sh -c "psql -c \"DROP TABLE x\""`: el escape rompia el pareo y el literal
    #     interno quedaba invisible (era el hueco 6 del header).
    # Un escaner IZQUIERDA-A-DERECHA respeta lo que bash respeta: la comilla que abre
    # PRIMERO manda, y adentro de "" el backslash escapa " \ $ ` y el newline.
    out, i, n = [], 0, len(texto)
    while i < n:
        c = texto[i]
        if c == "\\" and i + 1 < n:
            out.append(texto[i:i + 2])
            i += 2
            continue
        if c == "\x27":
            j = texto.find("\x27", i + 1)
            if j < 0:  # comilla sin cerrar: no es un literal, es texto
                out.append(c)
                i += 1
                continue
            lits.append(texto[i + 1:j])
            out.append("\x01%d\x01" % (len(lits) - 1))
            i = j + 1
            continue
        if c == "\x22":
            j, buf, cerrado = i + 1, [], False
            while j < n:
                if texto[j] == "\\" and j + 1 < n:
                    # dentro de "" el backslash SOLO escapa estos; con el resto se queda
                    buf.append(texto[j + 1] if texto[j + 1] in "\x22\\$`\n"
                               else texto[j:j + 2])
                    j += 2
                    continue
                if texto[j] == "\x22":
                    cerrado = True
                    break
                buf.append(texto[j])
                j += 1
            if not cerrado:
                out.append(c)
                i += 1
                continue
            lits.append("".join(buf))
            out.append("\x01%d\x01" % (len(lits) - 1))
            i = j + 1
            continue
        out.append(c)
        i += 1
    return "".join(out)


def sin_comentarios(texto):
    # Un # que abre palabra comenta hasta el fin de linea: NADA de ahi se ejecuta. Se
    # strippea DESPUES del enmascarado, asi un # dentro de comillas ya es un placeholder.
    # Esto es lo que convertia `rm -rf $PWD # PWD=tmp` en una asignacion falsa que
    # sobrescribia $PWD, y de paso evita el falso positivo simetrico (`ls # rm -rf /`).
    return re.sub(r"(?:(?<=^)|(?<=[\s;&|(]))#[^\n]*", "", texto, flags=re.M)


def sin_continuaciones(texto):
    return re.sub(r"\\\n", " ", texto)


def pelar(toks, lits):
    # Consume EN LOOP asignaciones (FOO=1), gramatica de shell (then/do/{) y wrappers
    # con el VALOR de sus flags. Antes era un regex de una pasada que consumia la flag
    # pero no su valor: sudo -u www-data rm -rf pasaba.
    i, cambio = 0, True
    while cambio and i < len(toks):
        cambio = False
        while i < len(toks) and (re.match(r"^\w+=", toks[i]) or toks[i] in DESCARTES):
            i, cambio = i + 1, True
        if i < len(toks) and base(limpiar(toks[i], lits)) in WRAPPERS:
            w = base(limpiar(toks[i], lits))
            i, cambio = i + 1, True
            while i < len(toks):
                t = toks[i]
                if t == "--":
                    i += 1
                    break
                if t.startswith("-") and len(t) > 1:
                    i += 1
                    if "=" not in t and t in WRAP_VALOR and i < len(toks):
                        i += 1
                    continue
                if w == "timeout" and re.match(r"^\d+(?:\.\d+)?[smhd]?$", t):
                    i += 1
                    continue
                if w == "env" and re.match(r"^\w+=", t):
                    i += 1
                    continue
                break
    return toks[i:]


def dividir(texto):
    # Los segmentos CRUDOS, en orden. Es el mismo corte que consume segmentos() y el
    # que consume el mapa de asignaciones: el indice tiene que significar lo mismo en
    # los dos, porque una asignacion solo vale para los segmentos POSTERIORES.
    texto = sin_continuaciones(texto)
    # TERCERA RONDA: `(rm -rf .)` no disparaba porque el token era "(rm". El parentesis
    # que abre en POSICION DE COMANDO se separa; el de $( no, porque ahi el $ lo precede
    # y esta clase no lo incluye (si se separara, $(pwd) y $(mktemp -d) se romperian).
    texto = re.sub(r"(^|[\s;&|])\(", r"\1( ", texto, flags=re.M)
    return re.split(r"\|\||&&|[;|\n&]", texto)


def segmentos(texto, lits):
    for i, seg in enumerate(dividir(texto)):
        toks = pelar(seg.split(), lits)
        if toks:
            yield i, toks


def desenvolver(toks, lits):
    # Devuelve (cuerpo, envuelto, remoto). envuelto: no se exige que rm sea el primer
    # token. remoto: no hay forma de resolver el radio del otro lado.
    c0 = base(limpiar(toks[0], lits))
    subs = REMOTAS_SUB.get(c0)
    if subs:
        for i, t in enumerate(toks[1:], 1):
            if t in subs:
                return pelar(toks[i + 1:], lits), True, True
        return toks, False, False
    if c0 in REMOTAS:
        return pelar(toks[1:], lits), True, True
    if c0 in LOCALES:
        return pelar(toks[1:], lits), True, False
    return toks, False, False


def cwd_tras_cd(toks, lits, ctx, cwd_actual):
    # Devuelve el cwd que rige DESPUES de este segmento. None cuando hay un cd que no se
    # puede resolver: un objetivo relativo con cwd desconocido tiene que escalar, no
    # compararse contra el cwd viejo.
    if not toks or base(limpiar(toks[0], lits)) != "cd":
        return cwd_actual
    args = [t for t in toks[1:] if not limpiar(t, lits).startswith("-")]
    if not args:
        return ctx["home"] or cwd_actual
    d = pre_normalizar(limpiar(args[0], lits), ctx)
    if certeza(d) != "literal":
        return None
    return norm(d if d.startswith("/") else os.path.join(cwd_actual or ".", d))


def posiciones_rm(toks, envuelto, lits, ctx=None):
    # el NOMBRE del comando tambien puede venir de una variable (RM=rm; $RM -rf .), y
    # comparar el token crudo contra "rm" lo dejaba pasar. Se resuelve igual que un
    # objetivo, con el mapa posicional del segmento.
    def es_rm(t):
        t = limpiar(t, lits)
        if base(t) == "rm":
            return True
        return ctx is not None and base(pre_normalizar(t, ctx)) == "rm"
    if envuelto:
        return [i for i, t in enumerate(toks) if es_rm(t)]
    return [0] if es_rm(toks[0]) else []


def fin_de_valor(s, i):
    # Donde termina el valor de una asignacion. Respeta $( ) para que d=$(mktemp -d) sea
    # UN valor y no "d=$(mktemp" mas un token suelto.
    prof, n = 0, len(s)
    while i < n:
        c = s[i]
        if s[i:i + 2] == "$(":
            prof += 1
            i += 2
            continue
        if c == "(" and prof:
            prof += 1
        elif c == ")" and prof:
            prof -= 1
        elif not prof and (c.isspace() or c in ";&|"):
            break
        i += 1
    return i


def asignaciones_de(seg, lits):
    # Las asignaciones REALES de un segmento, con la semantica de bash:
    #
    #   1. tienen que estar al PRINCIPIO del segmento (tras un export/declare opcional);
    #   2. si despues de ellas queda un COMANDO, son un prefijo de ENTORNO y NO cambian
    #      las variables del shell: bash expande los argumentos ANTES de armar ese
    #      entorno, asi que `PWD=tmp rm -rf $PWD` borra el cwd REAL. Se descartan.
    #
    # El barrido viejo era re.findall de (\w+)=(\S*) sobre el TEXTO ENTERO, sin distinguir
    # una asignacion de un comentario, de un argumento ni de un prefijo de entorno. Eso
    # daba un mapa que APAGABA el guard: `rm -rf $PWD # PWD=tmp`, `echo PWD=tmp; rm -rf
    # $PWD` y `PWD=tmp rm -rf $PWD` pasaban los tres.
    vals, i, n = {}, 0, len(seg)
    while True:
        m = re.compile(r"\s*(?:(?:export|declare|local|readonly|typeset)\s+)*"
                       r"(\w+)=").match(seg, i)
        if not m:
            break
        j = fin_de_valor(seg, m.end())
        vals[m.group(1)] = limpiar(seg[m.end():j], lits)
        i = j
    if seg[i:].strip():
        return {}  # prefijo de entorno: no toca el shell
    # PROTEGIDAS: ninguna asignacion puede redefinir con que se mide el radio.
    return {k: v for k, v in vals.items() if v and k not in PROTEGIDAS}


def mapa_por_segmento(texto, lits, base_vars):
    # Para cada segmento, el mapa VIGENTE justo antes de ejecutarlo. Con esto una
    # asignacion POSTERIOR ya no resuelve un objetivo anterior (`rm -rf $D; D=/tmp/x`
    # queda indecidible, que es lo correcto: cuando el rm corre, D no vale nada).
    acc, salida = dict(base_vars or {}), []
    for seg in dividir(texto):
        salida.append(dict(acc))
        for k, v in asignaciones_de(seg, lits).items():
            # si la misma var se asigna dos veces distinto, queda sin resolver y manda
            # el fail-closed
            acc[k] = None if (k in acc and acc[k] != v) else v
        acc = {k: v for k, v in acc.items() if v}
    salida.append(dict(acc))  # el estado FINAL: lo usa el chequeo del automarker
    return salida


def pre_normalizar(t, ctx):
    t = t.strip().strip("\x22\x27")
    # TERCERA RONDA: el escape INTERIOR. En una palabra sin comillas bash resuelve \X a X
    # (verificado read-only: echo .gi\t imprime .git), y el guard lo comparaba como texto,
    # asi que .gi\t y ./\.git no se reconocian como el .git. Va aca y no en limpiar()
    # porque limpiar() tambien procesa el token de COMANDO y el \; de find -exec.
    t = re.sub(r"\\(.)", r"\1", t)
    # las vars del propio comando PRIMERO: asi una cadena H=$HOME; rm -rf $H termina
    # resolviendo a $HOME y de ahi al path real
    vals = ctx.get("vars") or {}
    if vals:
        t = re.sub(r"\$\{(\w+)\}|\$(\w+)",
                   lambda m: vals.get(m.group(1) or m.group(2), m.group(0)), t)
    t = re.sub(r"\$\((?:pwd|PWD)\)|`pwd`", "$PWD", t)
    t = re.sub(r"^\$\{PWD\}", "$PWD", t)
    if not ctx["remoto"]:
        # ~+ es literalmente $PWD (verificado: echo ~+ imprime el cwd). Le faltaba, y
        # `rm -rf ~+` pasaba como si fuera un nombre de archivo raro.
        t = re.sub(r"^~\+(?=/|$)", ctx["cwd"] or ".", t)
        t = re.sub(r"^\$PWD(?=/|$)", ctx["cwd"] or ".", t)
        if ctx["home"]:
            t = re.sub(r"^~(?=/|$)", ctx["home"], t)
            t = re.sub(r"^\$\{?HOME\}?(?=/|$)", ctx["home"], t)
    return t


def certeza(o):
    # Que tan resoluble es el objetivo. Es el eje de la TERCERA RONDA: el guard resolvia
    # lo que podia y, cuando NO podia, concluia "no es catastrofico" — fail-OPEN. Ahora la
    # clase decide, y solo "literal" habilita comparar paths como texto.
    #
    #   opaco  el valor no se conoce: $VAR sin resolver, $(...), backticks, ~- ($OLDPWD),
    #          ~usuario (el home de OTRO), ${VAR:-x} con operador.
    #   glob   el valor lo produce BASH expandiendo metacaracteres (* ? [ ] { }).
    #   literal el path es exactamente lo que dice.
    if "$" in o or "`" in o or o.startswith("~"):
        return "opaco"
    return "glob" if any(c in o for c in GLOB) else "literal"


def flags_objetivos(args, lits, ctx):
    rec, objetivos, incierto = False, [], False
    for a in args:
        t = limpiar(a, lits)
        if t in ("-", "--", "+", "{}", ";", ")", "\\;"):
            continue
        # TERCERA RONDA: pre_normalizar va ANTES de clasificar. Con el orden invertido,
        # `F=-rf; rm $F .` caia en objetivos (no empieza con "-" TODAVIA), rec quedaba en
        # False y la rama entera del radio se salteaba: era rm -rf del cwd, y pasaba.
        p = pre_normalizar(t, ctx)
        if p.startswith("--"):
            if p in ("--recursive", "--recursive=true"):
                rec = True
            continue
        if p.startswith("-") and len(p) > 1 and not p[1].isdigit():
            if "r" in p[1:] or "R" in p[1:]:
                rec = True
            continue
        objetivos.append(p)
        if certeza(p) != "literal":
            incierto = True
    return rec, objetivos, incierto


def igual_o_ancestro(a, b):
    if not a or not b:
        return False
    a = a.rstrip("/") or "/"
    b = b.rstrip("/") or "/"
    return a == "/" or a == b or b.startswith(a + "/")


def bajo(a, prefijos):
    a = a.rstrip("/") or "/"
    return any(a == p or a.startswith(p + "/") for p in prefijos)


def rutinario(a):
    b = base(a.rstrip("/"))
    return b in RUTINA or b.startswith("coverage")


def sufijo_concreto(o):
    # $DIR/build no puede SER el proyecto ni un ancestro: hay al menos un componente
    # concreto despues de la expansion. $DIR solo, si.
    partes = o.split("/")
    ult = max([i for i, p in enumerate(partes) if "$" in p or "`" in p] or [-1])
    cola = [p for p in partes[ult + 1:] if p]
    if not cola:
        return False
    # TERCERA RONDA: antes solo se exigia que la cola no fuera . ni .. ni llevara *. Con
    # eso ${DIR:-/} contaba como sufijo concreto (su cola era "}") y el fail-closed no
    # corria; lo mismo ~usuario. Ahora la cola tiene que ser un nombre LITERAL.
    return all(re.match(r"^[\w.@+-]+$", p) and p not in ("..", ".") for p in cola)


def ancestros(p):
    out, p = [], (p or "").rstrip("/")
    while p and p != "/":
        out.append(p)
        p = os.path.dirname(p)
    out.append("/")
    return out


def candidatos_radio(ctx):
    # El conjunto EXACTO de paths cuyo borrado radio() considera catastrofico. Se usa para
    # decidir si un GLOB puede expandir a alguno de ellos, en vez de compararlo como texto.
    c = ["/"]
    for p in (ctx["cwd"], ctx["raiz"], ctx["home"]):
        if p:
            c.extend(ancestros(p))
            c.append(os.path.join(p, ".git"))
    for r in RAICES_APP:
        c.extend(ancestros(r))
    return set(c)


def glob_cuerpo(g):
    # glob de shell -> regex. * no cruza barras, ** si, {a,b} es alternancia, [!x] niega.
    out, i, n = [], 0, len(g)
    while i < n:
        c = g[i]
        if c == "*":
            out.append(".*" if g[i:i + 2] == "**" else "[^/]*")
            i += 2 if g[i:i + 2] == "**" else 1
            continue
        if c == "?":
            out.append("[^/]")
            i += 1
            continue
        if c == "[":
            j = g.find("]", i + 2)
            if j > 0:
                cuerpo = g[i + 1:j]
                if cuerpo[:1] in ("!", "^"):
                    cuerpo = "^" + cuerpo[1:]
                out.append("[" + cuerpo + "]")
                i = j + 1
                continue
        if c == "{":
            j = g.find("}", i + 1)
            if j > 0:
                out.append("(?:" + "|".join(glob_cuerpo(a) for a in g[i + 1:j].split(","))
                           + ")")
                i = j + 1
                continue
        out.append(re.escape(c))
        i += 1
    return "".join(out)


def alcanza_radio(o, ctx):
    a = norm(o if o.startswith("/") else os.path.join(ctx["cwd"] or ".", o))
    try:
        rx = re.compile("^" + glob_cuerpo(a) + "$")
    except re.error:
        return True  # un patron que no se puede modelar no se declara benigno
    return any(rx.match(c) for c in candidatos_radio(ctx))


def radio_glob(o, ctx):
    # TERCERA RONDA. El guard solo trataba el glob trailing /*, asi que `.g*` (= .git
    # .github .gitignore), `.*`, `.gi?`, `{.git,dist}` y `/ap*` le parecian nombres
    # literales y pasaban. Y el ruido va del otro lado: node_modules/* o coverage* NO
    # pueden escalar. Las dos reglas juntas dan las dos cosas:
    #   1. D/* es "vaciar D", que equivale a borrar D (asi ya disparaban /*, ./* y ../*);
    #   2. el patron se convierte a regex y se prueba contra candidatos_radio(): si PUEDE
    #      expandir al cwd, al proyecto, a un ancestro o al .git, escala; si su prefijo
    #      literal ya lo encierra en un subdirectorio acotado, no matchea nada y pasa.
    d = re.sub(r"/\*+$", "/", o)
    if d in ("*", "*/"):
        d = "."
    if certeza(d) == "literal" and radio(d, ctx):
        return "radio"
    return "radio" if alcanza_radio(o, ctx) else ""


def radio(o, ctx):
    # ¿el objetivo de un rm RECURSIVO tiene radio catastrofico?
    o = (o or "").strip()
    if not o:
        return False
    g = re.sub(r"/\*+$", "/", o)
    if g in ("*", "*/"):
        g = "."
    # el .git ENTERO (no un archivo de adentro: rm -rf .git/index.lock es rutina) es el
    # radio maximo posible — se lleva puesto justo lo que hace recuperable a todo lo demas
    if base(g.rstrip("/")) == ".git":
        return True
    if ctx["remoto"]:
        if g in (".", "./", "$PWD", "$PWD/", "${PWD}"):
            return True
        if g in ("~", "~/", "$HOME", "$HOME/", "${HOME}", "${HOME}/"):
            return True
        if g == ".." or g == "../" or g.startswith("../"):
            return True
        if not g.startswith("/"):
            return False
        a = norm(g)
        if a == "/":
            return True
        if re.match(r"^/home(?:/[^/]+)?/?$", a):
            return True
        if rutinario(a) or bajo(a, EFIMEROS):
            return False
        # container/host remoto: cualquier absoluto puede ser un bind mount del repo
        return True
    if g in ("$PWD", "$PWD/", "${PWD}", "$HOME", "$HOME/", "${HOME}", "~", "~/"):
        return True
    a = norm(g if g.startswith("/") else os.path.join(ctx["cwd"] or ".", g))
    if a == "/":
        return True
    if igual_o_ancestro(a, ctx["cwd"]):
        return True
    if ctx["raiz"] and igual_o_ancestro(a, ctx["raiz"]):
        return True
    if ctx["home"] and igual_o_ancestro(a, ctx["home"]):
        return True
    if re.match(r"^/home(?:/[^/]+)?/?$", a):
        return True
    if any(igual_o_ancestro(a, r) for r in RAICES_APP):
        return True
    # TERCERA RONDA: /srv/<app> y /var/www/<sitio> SON la app desplegada, no un
    # subdirectorio cualquiera de ella — el evasor medido era `sudo -u www-data rm -rf
    # /srv/domain`, que es exactamente el deploy de este repo. Un hijo de RUTINA
    # (/app/dist, /app/node_modules) sigue siendo rutina y no llega aca.
    return os.path.dirname(a.rstrip("/") or "/") in RAICES_APP and not rutinario(a)


def trackeado(a, ctx):
    d = os.path.dirname(a) or (ctx["cwd"] or ".")
    try:
        r = subprocess.run(["git", "-C", d, "ls-files", "--error-unmatch", "--", base(a)],
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=3)
        return r.returncode == 0
    except Exception:
        return False


def sensible_path(o, ctx):
    b = base(o).lower()
    if not b or b in (".", ".."):
        return False
    if re.search(EJEMPLAR, b):
        return False
    if not any(re.search(p, b) for p in SENSIBLES):
        return False
    if not ctx["remoto"]:
        a = norm(o if o.startswith("/") else os.path.join(ctx["cwd"] or ".", o))
        # un secreto suelto en /tmp es una COPIA de scratch, no el original del repo
        # (falso positivo medido: rm /tmp/no-secret.txt). Se exige que NO cuelgue del
        # cwd ni del HOME, porque un cwd bajo /tmp sigue siendo el proyecto parado.
        if bajo(a, EFIMEROS) and not bajo(a, [p for p in (ctx["cwd"], ctx["home"]) if p]):
            return False
        # trackeado en git: git checkout -- lo recupera y manda el git-guard
        if trackeado(a, ctx):
            return False
    return True


def rm_peligroso(args, lits, ctx):
    # EL PRINCIPIO (tercera ronda): si el objetivo de un rm RECURSIVO no es un literal que
    # se pueda resolver con CERTEZA, se ESCALA. Antes el guard intentaba resolver y, cuando
    # no podia, concluia "no es catastrofico" — fail-OPEN, y era la causa unica de los tres
    # bugs que el juez encontro.
    rec, objetivos, incierto = flags_objetivos(args, lits, ctx)
    # un rm recursivo SIN objetivo explicito lo recibe por stdin (echo . | xargs rm -rf):
    # el objetivo no esta en el texto, asi que no hay nada que resolver y se escala.
    # Excepcion: en `find ... -exec rm -rf {} +` el objetivo lo pone find y es find_peligroso
    # quien mira sus filtros — sin esto, borrar node_modules con find pasaba a ser ruido.
    if rec and not objetivos and not any("{}" in limpiar(a, lits) for a in args):
        return "radio-indecidible"
    if rec or incierto:
        for o in objetivos:
            clase = certeza(o)
            if clase == "opaco":
                # del otro lado del ssh/docker no hay cwd, pero $PWD/$HOME/~ siguen
                # significando algo catastrofico: radio() ya los cubre.
                if ctx["remoto"] and radio(o, ctx):
                    return "radio"
                # un valor derivado de mktemp es un directorio NUEVO por construccion:
                # no puede ser el proyecto ni un ancestro
                if "mktemp" in o:
                    continue
                # fail-closed: no se puede resolver el objetivo de un rm recursivo
                if rec and not sufijo_concreto(o):
                    return "radio-indecidible"
                continue
            if clase == "glob":
                # un glob NO recursivo borra archivos de un directorio, no el arbol:
                # rm -f *.log es rutina y no puede costar un disparo.
                if not rec:
                    continue
                motivo = ("radio" if radio(o, ctx) else "") if ctx["remoto"] \
                    else radio_glob(o, ctx)
                if motivo:
                    return motivo
                continue
            if radio(o, ctx):
                return "radio"
    for o in objetivos:
        if certeza(o) != "literal":
            continue
        if sensible_path(o, ctx):
            return "sensible"
    return ""


def find_peligroso(toks, lits, ctx):
    if base(limpiar(toks[0], lits)) != "find":
        return ""
    resto = toks[1:]
    raices = []
    for t in resto:
        if t.startswith("-") or t in ("(", "!", ")"):
            break
        raices.append(t)
    borra, filtrado = False, False
    for i, t in enumerate(resto):
        if t == "-delete":
            borra = True
        if t in ("-exec", "-execdir", "-ok", "-okdir") and any(
                base(limpiar(x, lits)) in ("rm", "shred", "unlink")
                for x in resto[i + 1:i + 3]):
            borra = True
        if t in ("-name", "-iname", "-path", "-ipath", "-regex", "-wholename"):
            pat = limpiar(resto[i + 1], lits) if i + 1 < len(resto) else ""
            if pat not in ("*", ".*", ""):
                filtrado = True
    if not borra or filtrado:
        return ""
    for r in raices:
        if radio(pre_normalizar(limpiar(r, lits), ctx), ctx):
            return "radio"
    return ""


def sql_destructivo(sql):
    # CAMBIO 2.8: /*…*/ primero — DROP/**/TABLE evadia y un /* WHERE */ comentado
    # apagaba la regla.
    limpio = re.sub(r"/\*[\s\S]*?\*/", " ", sql)
    limpio = re.sub(r"--[^\n]*", " ", limpio)
    for st in limpio.split(";"):
        if re.search(r"\bDROP\s+(?:DATABASE|TABLE|SCHEMA|OWNED\s+BY|ROLE|USER|INDEX"
                     r"|VIEW|MATERIALIZED\s+VIEW|TYPE|EXTENSION|SEQUENCE|FUNCTION"
                     r"|TRIGGER|TABLESPACE)\b", st, re.I):
            return True
        if re.search(r"\bALTER\s+TABLE\b[\s\S]*?\bDROP\s+(?:COLUMN|CONSTRAINT)\b", st, re.I):
            return True
        if re.search(r"\bTRUNCATE\b", st, re.I):
            return True
        # WHERE true / WHERE 1=1 no filtra NADA: es un DELETE completo con disfraz
        sw = re.sub(r"\bWHERE\s+(?:true|1\s*=\s*1)\b", " ", st, flags=re.I)
        if re.search(r"\bWHERE\b", sw, re.I):
            continue
        if re.search(r"\bDELETE\s+FROM\b", sw, re.I):
            return True
        if re.search(r"\bUPDATE\b[\s\S]*?\bSET\b", sw, re.I):
            return True
    return False


def literales_de(toks, lits):
    idx = re.findall("\x01(\\d+)\x01", " ".join(toks))
    return [lits[int(i)] for i in idx if int(i) < len(lits)]


def tiene_cliente(parte, lits):
    # El cliente tiene que estar en POSICION DE COMANDO (mismo criterio que
    # DOMAINSERV-146 para rm). Con any(token) alcanzaba que la palabra apareciera:
    # `docker ps | grep -i mysql` y `which mysql` disparaban sql-opaco.
    toks = pelar(parte.split(), lits)
    if not toks:
        return False
    cuerpo, envuelto, _ = desenvolver(toks, lits)
    if not cuerpo:
        return False
    if base(limpiar(cuerpo[0], lits)) in SQL_CLIENTES:
        return True
    return envuelto and any(base(limpiar(t, lits)) in SQL_CLIENTES for t in cuerpo)


def sql_de_archivo(path, ctx):
    # El archivo que el cliente SQL va a ejecutar: si se puede LEER, se analiza (mucho
    # mejor que escalar a ciegas); si no, se escala. `-f -` es stdin y lo cubre el
    # analisis del pipeline. Con ctx remoto el path es del OTRO lado: no se lee.
    if path in ("-", "/dev/stdin"):
        return ""
    if ctx["remoto"]:
        return "sql-opaco"
    p = pre_normalizar(path, ctx)
    if "$" in p or "`" in p or "*" in p:
        return "sql-opaco"
    a = p if p.startswith("/") else os.path.join(ctx["cwd"] or ".", p)
    try:
        if os.path.getsize(a) > 262144:
            return "sql-opaco"
        with open(a, errors="replace") as f:
            texto = f.read()
    except Exception:
        return "sql-opaco"
    return "sql" if sql_destructivo(texto) else ""


def archivos_sql(tramo, toks, lits):
    paths = []
    for m in re.finditer(r"(?<!<)<(?!<)\s*([^\s;|&<>]+)", tramo):
        paths.append(limpiar(m.group(1), lits))
    for i, t in enumerate(toks):
        if not any(base(x) == "psql" for x in toks):
            break
        if t in ("-f", "--file") and i + 1 < len(toks):
            paths.append(limpiar(toks[i + 1], lits))
        elif t.startswith("--file="):
            paths.append(limpiar(t.split("=", 1)[1], lits))
    return paths


def sql_en_pipeline(texto, lits, ctx):
    # CAMBIO 2.8: el PIPELINE completo. Antes se miraba segmento por segmento y el
    # pipe los separaba: echo "DROP TABLE x" | psql pasaba.
    for tramo in re.split(r"&&|\|\||[;\n]", sin_continuaciones(texto)):
        partes = tramo.split("|")
        con = [i for i, p in enumerate(partes) if tiene_cliente(p, lits)]
        if not con:
            continue
        toks = tramo.split()
        adentro = literales_de(toks, lits)
        if any(sql_destructivo(s) for s in adentro):
            return "sql"
        # el archivo que ejecuta el cliente: se LEE si se puede, y si no se escala
        for p in archivos_sql(tramo, toks, lits):
            m = sql_de_archivo(p, ctx)
            if m:
                return m
        if any(re.search(r"\\ir?\s", s) for s in adentro):
            return "sql-opaco"
        if con[0] > 0:
            arriba = partes[0].split()
            if arriba and base(limpiar(arriba[0], lits)) not in ("echo", "printf"):
                return "sql-opaco"
    return ""


def escribe_automarker(texto, lits, ctx):
    # CAMBIO 3: el agente podia escribirse el marker de bypass con un echo >. Autorizar
    # un borrado tiene que costar una intervencion HUMANA, no una llamada de Bash.
    def apunta(t):
        t = expandir(t, lits)
        # TERCERA RONDA: el path puede venir ENTERO en una variable, y ahi el literal
        # "destructive-bypass" no esta en la linea — M=$HOME/...-bypass-x; echo r > $M.
        return "destructive-bypass" in t or "destructive-bypass" in pre_normalizar(t, ctx)
    for m in re.finditer(r">>?\s*([^\s;|&<>]+)", texto):
        if apunta(m.group(1)):
            return True
    for _, toks in segmentos(texto, lits):
        c0 = base(limpiar(toks[0], lits))
        # un interprete escribe desde su propio literal, sin pasar por una redireccion:
        # python3 -c "open(...,\x27w\x27)". grep/ls NO estan en estas listas, asi que
        # inspeccionar el marker sigue siendo gratis.
        if c0 in VERBOS_ESCRITURA or c0 in INTERPRETES_ESCRITURA:
            if any(apunta(t) for t in toks[1:]):
                return True
    return False


def hay_interprete(toks, lits):
    for i, t in enumerate(toks):
        b = base(limpiar(t, lits))
        if b == "eval":
            return True
        if b in SHELLS and any(re.match(r"^-\w*c$", limpiar(x, lits))
                               for x in toks[i + 1:i + 4]):
            return True
    return False


def destructivo(texto, lits=None, hondura=0, ctx=None):
    lits = [] if lits is None else lits
    ctx = ctx_base() if ctx is None else ctx
    texto = sin_comentarios(enmascarar(texto, lits))
    # El mapa de asignaciones es POSICIONAL: mapas[i] es lo que vale justo antes del
    # segmento i, y mapas[-1] el estado final. Sin esto una asignacion POSTERIOR resolvia
    # un objetivo anterior (rm -rf $D; D=/tmp/x pasaba como si D valiera /tmp/x).
    mapas = mapa_por_segmento(texto, lits, ctx.get("vars"))
    final = dict(ctx, vars=mapas[-1])
    if escribe_automarker(texto, lits, final):
        return "automarker"
    motivo = sql_en_pipeline(texto, lits, final)
    if motivo:
        return motivo
    # el cwd puede cambiar DENTRO del mismo comando: cd <padre> && rm -rf <proyecto> es el
    # proyecto entero, escrito como nombre relativo. Se sigue el cd cuando el destino es
    # literal, y si no se puede resolver el cwd queda en None -> los relativos escalan.
    cwd_vivo = ctx["cwd"]
    for idx, toks in segmentos(texto, lits):
        cur = dict(ctx, cwd=cwd_vivo, vars=mapas[idx] if idx < len(mapas) else mapas[-1])
        cwd_vivo = cwd_tras_cd(toks, lits, cur, cwd_vivo)
        cuerpo, envuelto, remoto = desenvolver(toks, lits)
        if not cuerpo:
            continue
        sub = dict(cur)
        sub["remoto"] = ctx["remoto"] or remoto
        for i in posiciones_rm(cuerpo, envuelto, lits, sub):
            motivo = rm_peligroso(cuerpo[i + 1:], lits, sub)
            if motivo:
                return motivo
        motivo = find_peligroso(toks, lits, cur)
        if motivo:
            return motivo
        adentro = literales_de(cuerpo, lits)
        # CAMBIO 2.1: recursar tambien en el literal de las ENVOLTURAS. El deploy de
        # este repo tiene la forma ssh vps "rm -rf /srv/domain" y pasaba porque solo
        # se recursaba con sh -c/eval.
        if hondura < 3 and adentro and (envuelto or hay_interprete(cuerpo, lits)):
            for s in adentro:
                motivo = destructivo(s, lits, hondura + 1, sub)
                if motivo:
                    return motivo
    return ""

print(destructivo(sys.stdin.read()))
' 2>/dev/null)
  if [ -n "$destructivo" ]; then
    destr_bypass="$HOME/.local/state/domain/destructive-bypass-$session_id"
    # El bypass NO habilita escribirse el propio bypass: si lo hiciera, un agente que
    # ya consumió uno podría renovarlo solo. automarker es siempre humano-only.
    if [ -r "$destr_bypass" ] && [ "$destructivo" != "automarker" ]; then
      destr_razon=$(head -1 "$destr_bypass" 2>/dev/null)
      rm -f "$destr_bypass"
      type domain_log_injection >/dev/null 2>&1 && \
        domain_log_injection "PreToolUse" "$session_id" \
          "destructive-guard bypass consumido ($destructivo): ${destr_razon:-sin razón declarada}"
    else
      case "$destructivo" in
        radio)
          destr_detalle="borrado RECURSIVO cuyo objetivo es el directorio donde estás parado, la raíz del repo, un ANCESTRO de cualquiera de los dos, /, \$HOME, el .git, o una raíz de app de container (/app /srv /repo /workspace /var/www /usr/src/app — el incidente fue justo eso: /app era un bind mount del repo del host y no lo parecía). Un rm -rf de node_modules, dist, build, vendor o cualquier subdirectorio que NO sea el proyecto entero NO llega acá." ;;
        radio-indecidible)
          destr_detalle="borrado RECURSIVO cuyo objetivo no se puede resolver (lleva \$VAR, \$(…) o backticks sin sufijo concreto). El guard NO asume que es inofensivo: si la variable vale / o \$HOME el daño es total, así que escala." ;;
        sensible)
          destr_detalle="rm de un archivo SENSIBLE que NO está trackeado en git (.env*, *.key, *.pem, id_rsa, *credential*, *secret*, *.p12) — es el incidente original, \`docker exec <ctr> rm -f .env.qa\`. Un archivo trackeado no llega acá: ese lo recupera git checkout -- y lo cubre el git-guard." ;;
        sql)
          destr_detalle="SQL destructivo (DROP DATABASE/TABLE/SCHEMA/ROLE/INDEX/VIEW/TYPE/EXTENSION/OWNED BY, ALTER TABLE … DROP COLUMN, TRUNCATE, DELETE/UPDATE sin WHERE real — WHERE true y WHERE 1=1 cuentan como SIN where)." ;;
        sql-opaco)
          destr_detalle="un cliente SQL que ejecuta un archivo o un stdin que el guard NO puede leer (-f, <, \\i, un pipe que no es echo). No se puede afirmar que sea benigno, así que escala." ;;
        automarker)
          destr_detalle="el comando ESCRIBE el marker de bypass de este guard. Autorizar un borrado irreversible tiene que costar una intervención HUMANA fuera del agente, no una llamada de Bash del agente. Ningún bypass habilita esta operación: pedíselo al humano." ;;
      esac
      # acceptEdits es INTERACTIVO (se activa con shift+tab, hay humano al teclado), así
      # que ahí un "ask" sí llega a una persona y el deny sería un muro sin salida. El
      # deny duro queda SOLO para bypassPermissions, donde nadie ve el diálogo. Un modo
      # DESCONOCIDO cae en ask: un modo nuevo de Claude Code no debe volverse deny mudo.
      case "$perm_mode" in
        bypassPermissions) destr_dec="deny" ;;
        *)                 destr_dec="ask" ;;
      esac
      if [ "$destructivo" = "automarker" ]; then
        emit_decision "$destr_dec" "domain destructive-guard (DOMAINSERV-222) [$destructivo]: $destr_detalle"
      else
        emit_decision "$destr_dec" "domain destructive-guard (DOMAINSERV-222) [$destructivo]: $destr_detalle Nada de esto se recupera con git. Si el borrado es legítimo, el HUMANO autoriza UNO SOLO con: echo 'tu razón' > $destr_bypass"
      fi
    fi
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

# DOMAINSERV-146: esto era un re.search de "git commit" sobre la línea ENTERA, así que
# `grep -nE "commit|git commit|tests-ok" archivo.js` —100% read-only— disparaba el
# commit-gate: el literal viajaba DENTRO del patrón de grep y la heurística lo leyó como
# el comando a ejecutar.
#
# Mismo linaje que DOMAINSERV-114, que le enseñó al GIT-GUARD que un heredoc y un string
# entrecomillado son DATOS y no comandos. El commit-gate había quedado sin ese
# tratamiento. Acá van las dos capas: strippear datos, y exigir que git sea el PRIMER
# token de algún comando de la lista. Con la primera alcanzaba para el caso reportado;
# la segunda es la que hace que la decisión no dependa de dónde caiga una comilla.
INTERPRETES = r"\b(?:bash|sh|zsh|dash|ksh)\s+(?:-\w+\s+)*-c\b|\beval\b|\bxargs\b"

# fail-closed: con un intérprete que EJECUTA el literal (sh -c "git commit"), lo
# entrecomillado SÍ es comando y no se strippea nada
if not re.search(INTERPRETES, cmd):
    # DOMAINSERV-222: terminador de heredoc INDENTADO (<<-MSG con tabs) — ver (A2)
    cmd = re.sub(r"<<[-~]?\s*([\x27\x22]?)(\w+)\1[\s\S]*?^[ \t]*\2[ \t]*$", " LITERAL ", cmd, flags=re.M)
    cmd = re.sub(r"\x27[^\x27]*\x27", " LITERAL ", cmd)
    cmd = re.sub(r"\x22[^\x22]*\x22", " LITERAL ", cmd)

def ejecuta_git_commit(texto):
    # cada segmento de la lista/pipeline es un comando aparte: `cd x && git commit` sí,
    # `grep "git commit" f` no
    for seg in re.split(r"\|\||&&|[;|\n&]", texto):
        seg = re.sub(r"^(?:\w+=\S*\s+)+", "", seg.strip())
        seg = re.sub(r"^(?:sudo|command|env|time|nohup)\s+(?:-\w+\s+)*", "", seg)
        toks = seg.split()
        if not toks:
            continue
        if toks[0].rsplit("/", 1)[-1] != "git":
            continue
        resto = re.sub(
            r"^(?:-[cC]\s+\S+\s+|--(?:git-dir|work-tree)(?:=\S+|\s+\S+)?\s+)*",
            "", " ".join(toks[1:]))
        if re.match(r"commit\b", resto):
            return True
    return False

if ejecuta_git_commit(cmd):
    print("yes")
    sys.exit(0)

# el intérprete que ejecuta un literal tiene que seguir gateado: se mira ADENTRO
if re.search(INTERPRETES, cmd):
    for lit in re.findall(r"\x27([^\x27]*)\x27", cmd) + re.findall(r"\x22([^\x22]*)\x22", cmd):
        if ejecuta_git_commit(lit):
            print("yes")
            sys.exit(0)
' 2>/dev/null)
  if [ "$is_commit" = "yes" ]; then
    # DOMAINSERV-257: las policies existían en la BD y NADA las hacía llegar al camino
    # donde se violan. El único disparador vivo estaba en sdd-review —o sea, dentro de un
    # flow que LLEGA a esa fase— y aun ahí entregaba la primera línea truncada a 160
    # chars, no la regla. Medido: 8 commits directos a main después de abrir el ticket,
    # con la policy ramas-y-tags v3 activa y sus 4636 chars de cuerpo sin entregar nunca.
    #
    # Va por permissionDecisionReason y NO por additionalContext: el reason es campo
    # confirmado de PreToolUse, y este gate no puede depender de una capacidad que, si no
    # existe, falla sin que nadie se entere. Se paga una interrupción por sesión, en el
    # primer commit; los siguientes pasan por el marker de abajo.
    #
    # El body lo cachea SessionStart, que no declara timeout. Acá no se hace red: un
    # PreToolUse tiene 10 s y agotarlos BLOQUEA la tool que el usuario quiso correr.
    pol_marker="$HOME/.local/state/domain/policy-entregada-git_workflow-$session_id"
    pol_slug=$(basename "$(git rev-parse --show-toplevel 2>/dev/null || echo "${payload_cwd:-$PWD}")")
    pol_cache="$HOME/.local/state/domain/policy-bodies/$pol_slug/git_workflow.md"
    if [ ! -f "$pol_marker" ] && [ -s "$pol_cache" ]; then
      mkdir -p "$HOME/.local/state/domain" 2>/dev/null
      date +%s > "$pol_marker"
      emit_decision "ask" "POLICIES VIGENTES DE ESTE PROYECTO PARA git (kind=git_workflow).
Se entregan porque este comando es un commit, no porque se sospeche una violación.
Si lo que vas a commitear las contradice, decilo ANTES de ejecutarlo.

$(cut -c1-8000 "$pol_cache")"
    fi
    # Fail-OPEN deliberado si no hay cache (VPS caído en el SessionStart): el commit sigue
    # su curso normal. Negar por falta de base dejaría el gate insatisfacible y empuja al
    # bypass permanente, que es el modo de falla de DOMAINSERV-111/175/195.

    # DOMAINSERV-108: flows MICRO (ediciones triviales sin lógica testeable:
    # texto de front, script nuevo, doc/config) están EXENTOS del requisito de
    # tests (decisión explícita del usuario). El post-orchestrate escribe el
    # modo del flow como field3 del marker; si es "micro" el commit pasa sin
    # tests-ok. Cualquier otro modo mantiene el gate DOMAINSERV-74 intacto.
    flow_marker=$(domain_flow_marker_de_este_agente)
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
      # DOMAINSERV-219: con field3 presente manda SOLO field3. No es un OR con
      # field2: `git diff HEAD` no lista untracked, así que un .go nuevo sin
      # `git add` lo deja igual y aceptar cualquiera de los dos conservaría ese
      # falso verde. field2 solo se usa si field3 no está (marker ya en disco,
      # escrito por la versión anterior del post-test).
      stored_code=$(cut -f3 "$marker" 2>/dev/null)
      if [ -n "$stored_code" ] && type domain_tests_code_hash >/dev/null 2>&1; then
        current_code=$(domain_tests_code_hash)
        [ -n "$current_code" ] && [ "$current_code" = "$stored_code" ] && fresh="yes"
      else
        stored_hash=$(cut -f2 "$marker" 2>/dev/null)
        # DOMAINSERV-95: sin hash almacenado → NO fresco (fail-closed). Un marker
        # legacy (solo-timestamp) o forjado con printf ya no habilita el commit.
        if [ -n "$stored_hash" ]; then
          # DOMAINSERV-266: ver domain_sha256 en domain-hooks-lib.sh. En macOS, sha256sum
          # directo daba vacío y la comparación de abajo caía a fail-closed permanente.
          current_hash=$(git diff --no-color HEAD 2>/dev/null | domain_sha256)
          [ "$current_hash" = "$stored_hash" ] && fresh="yes"
        fi
      fi
      # DOMAINSERV-237: el hash prueba que el código no cambió DESPUÉS de la
      # corrida, no que la corrida lo haya EVALUADO. Con el orden normal del flujo
      # (editar → add → test → commit) el hash coincide igual aunque la corrida
      # haya sido de otro paquete. Falta cruzar el alcance recorrido (field4) con
      # los .go que se van a commitear.
      #
      # Solo aplica a los markers escritos por `go test` (field5="go"): los demás
      # runners conservan su alcance histórico de repo entero, y exigirle cobertura
      # por path a una suite de bash denegaría commits legítimos.
      if [ "$fresh" = "yes" ] && [ "$(cut -f5 "$marker" 2>/dev/null | head -1)" = "go" ]; then
        sin_cubrir=$(SCOPES="$(cut -f4 "$marker" 2>/dev/null | head -1)" python3 -c '
import os, subprocess, sys

scopes = [s for s in (os.environ.get("SCOPES") or "").split() if s]
if not scopes:
    # marker de go SIN alcance: fail-closed (mismo criterio que DOMAINSERV-95 con
    # el hash ausente). Un marker sin alcance no acredita cobertura de nada.
    print("(alcance no registrado)")
    sys.exit(0)

def sh(*a):
    try:
        return subprocess.run(a, capture_output=True, text=True, timeout=10).stdout.split("\n")
    except Exception:
        return []

raiz = (sh("git", "rev-parse", "--show-toplevel") or [""])[0].strip()
if not raiz:
    sys.exit(0)

cambiados = set()
for rel in sh("git", "diff", "--name-only", "HEAD") + sh("git", "ls-files", "--others", "--exclude-standard"):
    rel = rel.strip()
    if rel.endswith(".go"):
        cambiados.add(os.path.join(raiz, rel))

def cubierto(p):
    d = os.path.dirname(p)
    return any(d == s or d.startswith(s.rstrip("/") + "/") for s in scopes)

faltan = sorted(p[len(raiz) + 1:] for p in cambiados if not cubierto(p))
if faltan:
    print(" ".join(faltan[:5]))
' 2>/dev/null)
        if [ -n "$sin_cubrir" ]; then
          fresh=""
          gate_motivo_alcance="$sin_cubrir"
          # DOMAINSERV-245: el deny tiene que decir las DOS mitades —qué se corrió y qué
          # faltaba—. Con solo la segunda, quien recibe el deny no sabe si el problema es que
          # corrió la suite equivocada o que no corrió nada, y son arreglos distintos. Importa
          # especialmente con subagentes: el marker puede haberlo escrito otro, y su alcance es
          # el único dato que dice si esa corrida sirve para este commit.
          gate_alcance_corrido=$(cut -f4 "$marker" 2>/dev/null | head -1)
          # DOMAINSERV-245: field6 = origen. Un marker escrito antes de que este campo existiera
          # devuelve vacío, y eso se reporta como desconocido en vez de afirmar "main".
          gate_origen_corrida=$(cut -f6 "$marker" 2>/dev/null | head -1)
        fi
      fi
    fi
    # bypass de un solo uso que crea el humano: en modos automáticos el deny de
    # abajo es duro y el gate quedaba insatisfacible (DOMAINSERV-195)
    bypass="$HOME/.local/state/domain/gate-bypass-$session_id"
    if [ "$fresh" != "yes" ] && [ -r "$bypass" ]; then
      bypass_razon=$(head -1 "$bypass" 2>/dev/null)
      rm -f "$bypass"
      type domain_log_injection >/dev/null 2>&1 && \
        domain_log_injection "PreToolUse" "$session_id" \
          "commit-gate bypass consumido ($tool_name): ${bypass_razon:-sin razón declarada}"
      fresh="yes"
    fi
    if [ "$fresh" != "yes" ]; then
      case "$perm_mode" in
        default|plan) commit_dec="ask" ;;
        *)            commit_dec="deny" ;;
      esac
      # DOMAINSERV-265: el mensaje sale del stack REAL del repo. Hardcodear `go test`
      # en un repo Node hacía que se leyera "el gate no soporta este stack" y se fuera
      # al bypass con la suite ejecutable. Vacío = no lo reconocemos, y hay que decirlo:
      # un bypass informado vale más que uno por reflejo.
      gate_raiz=$(git rev-parse --show-toplevel 2>/dev/null)
      gate_cmd_tests=$(domain_test_cmd_sugerido "$gate_raiz")
      if [ -n "$gate_cmd_tests" ]; then
        gate_como_correr="Corré la suite con \`$gate_cmd_tests\`."
      else
        gate_como_correr="No pude determinar cómo se corren los tests en este repo (no encontré go.mod, package.json, pyproject.toml, Cargo.toml, composer.json ni Gemfile). Si tu suite es de otro tipo —por ejemplo scripts de shell— el gate no la reconoce y el bypass es la vía correcta."
      fi
      if [ -n "${gate_motivo_alcance:-}" ]; then
        emit_decision "$commit_dec" "domain commit-gate (DOMAINSERV-237): la corrida de tests NO cubrió estos archivos: ${gate_motivo_alcance}. Lo que SÍ se corrió fue: ${gate_alcance_corrido:-(alcance no registrado)}, y lo corrió: ${gate_origen_corrida:-(origen no registrado — marker anterior a DOMAINSERV-245)}. Que el código no haya cambiado desde la corrida no prueba que la corrida lo haya evaluado, y que el marker exista no prueba que su corrida cubra ESTE commit — puede haberlo escrito un subagente que corrió otra suite (DOMAINSERV-245). Corré la suite recursiva que los contiene, sin acotarla a un subconjunto. ${gate_como_correr} Si de verdad no se pueden correr acá, autorizá UN commit escribiendo la razón en un comando SEPARADO del git commit (con && el hook inspecciona antes de que exista el archivo y deniega igual): echo 'tu razón' > $bypass"
      else
        emit_decision "$commit_dec" "domain commit-gate (DOMAINSERV-74): no hay corrida de tests que cubra el estado actual del código. El marker tests-ok falta, expiró (30 min) o el working tree cambió después de los tests. ${gate_como_correr} Si los tests no se pueden correr acá (dependen de VPN, de un servicio externo, o el contenido ya viene testeado aguas arriba), autorizá UN commit escribiendo la razón en un comando SEPARADO del git commit (con && el hook inspecciona antes de que exista el archivo y deniega igual): echo 'tu razón' > $bypass"
      fi
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

# DOMAINSERV-218: el marker de ESTE agente. Para el hilo principal es el de siempre.
marker=$(domain_flow_marker_de_este_agente)
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
      #
      # DOMAINSERV-218 incremento 4: el marker v1 legacy NO PUEDE LLEVAR SCOPE —es
      # "<timestamp>\t<flow_run_id>\t<mode>"— así que autorizar por esta rama es autorizar SIN
      # restricción de path. Para el hilo principal eso es el comportamiento histórico y se
      # conserva. Para un SUBAGENTE es un agujero: MEDIDO el 2026-08-06, un subagente scopeado a
      # openspec/** escribió en .github/ porque su marker había degradado a legacy.
      #
      # Un subagente cae al gate en vez de recibir vía libre. NO es un deny: sigue pudiendo pedir
      # autorización, y el hilo principal no se toca, así que el gate no se vuelve insatisfacible.
      if echo "$resp" | grep -qE '"status":[[:space:]]*"(running|pending)"'; then
        if [ -z "${agent_id:-}" ]; then
          exit 0
        fi
        case "$perm_mode" in
          default|plan) legacy_dec="ask" ;;
          *)            legacy_dec="deny" ;;
        esac
        emit_decision "$legacy_dec" "domain gate SDD (DOMAINSERV-218): sos un subagente y tu marker de flow es v1 legacy, que no puede declarar allowed_paths. Autorizarte por esta vía te daría permiso de editar CUALQUIER path, que es justo lo que el aislamiento por agente evita. Pedí tu token con domain_flow_grant_token declarando tu allowed_paths, o dejá que la edición la haga el hilo principal."
        exit 0
      fi
      # no confirmado / server unreachable → fail-closed → gate
    elif [ -n "$field1" ]; then
      # v2: field1 = token HMAC → validar firma + flow activo server-side
      #
      # DOMAINSERV-218: el agente se manda SOLO si el marker que se está usando es el PROPIO.
      # Si se cayó al fallback del marker de sesión, el token es el del padre y no lleva
      # agente: mandar el nuestro haría que el server responda agent_mismatch y dejaría sin
      # editar justamente al subagente que el fallback existe para autorizar.
      agente_del_token=""
      case "$marker" in
        *"-${agent_id:-__sin_agente__}") agente_del_token="${agent_id:-}" ;;
      esac
      resp=$(domain_call_tool domain_flow_validate_token \
        "{\"token\":\"$field1\",\"session_id\":\"$session_id\",\"agent_id\":\"$agente_del_token\"}" 2>/dev/null)
      # vinfo = "<valid>\t<allowed_paths_json>\t<token_renovado>" — el campo 3 se agrega al
      # FINAL (DOMAINSERV-218): los lectores que usen cut -f1..f2 no se enteran, y un server
      # viejo que no renueva devuelve vacío ahí, que se trata como "no hubo renovación".
      vinfo=$(printf '%s' "$resp" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
    for c in d.get("result",{}).get("content",[]):
        if isinstance(c, dict) and c.get("type") == "text":
            body = json.loads(c["text"])
            if body.get("valid"):
                ap = body.get("allowed_paths") or []
                otros = "1" if body.get("scopes_de_otros") else ""
                print("yes\t" + json.dumps(ap) + "\t" + (body.get("token") or "") + "\t" + otros)
            break
except Exception:
    pass
' 2>/dev/null)
      valid=$(printf '%s' "$vinfo" | cut -f1)
      allowed_json=$(printf '%s' "$vinfo" | cut -f2)
      token_renovado=$(printf '%s' "$vinfo" | cut -f3)
      # DOMAINSERV-218: si al token le quedaba poca vigencia, el server devolvió uno nuevo. Se
      # reescribe el marker acá para que el TTL mida INACTIVIDAD: mientras el agente edite, su
      # autorización se corre sola y una fase larga no la pierde a mitad de camino.
      #
      # Un fallo escribiendo NO bloquea la edición que ya está autorizada: el peor caso es que
      # el token venza como antes de este cambio.
      if [ -n "$token_renovado" ] && [ "$token_renovado" != "$field1" ]; then
        nueva_exp=$(python3 -c '
from datetime import datetime, timezone, timedelta
print((datetime.now(timezone.utc) + timedelta(seconds=1800)).isoformat())
' 2>/dev/null)
        printf '%s\t%s\n' "$token_renovado" "$nueva_exp" > "$marker" 2>/dev/null || true
      fi
      scopes_de_otros=$(printf '%s' "$vinfo" | cut -f4)
      if [ "$valid" = "yes" ]; then
        # sin allowlist (flow normal) → sin restricción de path (backward-compat).
        if [ -z "$allowed_json" ] || [ "$allowed_json" = "[]" ] || [ "$allowed_json" = "null" ]; then
          # DOMAINSERV-218 incremento 5. MEDIDO instrumentando el hook: acá salía el subagente
          # que había caído al marker del PADRE. El token del padre no tiene allowed_paths, así
          # que esta rama le daba vía libre a todos los paths y el aislamiento no existía por
          # más scopes declarados que hubiera.
          #
          # Se acota a los flows donde el aislamiento está EN JUEGO: solo si OTRO agente tiene
          # territorio reservado. En un flow normal —sin partición— esta rama sigue autorizando
          # igual que siempre, que es lo que mantiene el gate satisfacible.
          if [ -n "${agent_id:-}" ] && [ -n "$scopes_de_otros" ]; then
            case "$perm_mode" in
              default|plan) herencia_dec="ask" ;;
              *)            herencia_dec="deny" ;;
            esac
            emit_decision "$herencia_dec" "domain gate SDD (DOMAINSERV-218): sos un subagente sin scope propio y estás usando el token del hilo principal, que no declara allowed_paths. En este flow OTRO agente sí reservó territorio, así que heredar una autorización sin restricción contradice esa partición. Pedí tu token con domain_flow_grant_token declarando tu allowed_paths."
            exit 0
          fi
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
    # DOMAINSERV-146: el here-doc solo cuenta como edición si ESCRIBE. Antes bastaba
    # con que hubiera una extensión de código en cualquier parte después del marcador,
    # así que un heredoc que solo MENCIONA un archivo lo disparaba: el
    # ssh host bash-s <<REMOTE ... grep/md5sum ... REMOTE del reporte era 100%
    # read-only y quedaba bloqueado por el gate SDD.
    #
    # (Sin comillas simples en estos comentarios: el bloque viaja dentro de un
    # python3 -c entrecomillado con comilla simple, y una sola cierra el string.)
    #
    # Las dos formas en que un heredoc escribe código de verdad:
    #   1. redirección/tee a un archivo de código EN LA MISMA línea del marcador
    #      (cat > f.go <<EOF / cat <<EOF > f.go / tee f.sh <<EOF)
    #   2. un intérprete que lee el script por stdin y escribe adentro
    #      (python3 - <<PY … open("x.go","w") … PY). Sin -c, los patrones de
    #      intérprete-en-línea de abajo no lo ven; esta es su contraparte por heredoc.
    r"(?:>>?\s*\S*" + code_ext + r"[^\n]*<<|<<[^\n]*>>?\s*\S*" + code_ext
        + r"|\btee\s+(?:-a\s+)?\S*" + code_ext + r"[^\n]*<<)",
    r"<<[-~]?\s*[\x27\x22]?\w+[\x27\x22]?[\s\S]*?"
        r"(?:open\s*\([^)]*[\x27\x22][wa]|write_text|writelines|writeFileSync"
        r"|appendFileSync|File\.\s*(?:write|open)|IO\.\s*(?:write|open))",
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
