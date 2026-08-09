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

# Lib compartida (best-effort): aporta domain_tests_code_hash. Si no está, el
# marker se escribe igual con el hash legacy — el gate lo tolera.
LIB="$(dirname "$0")/domain-hooks-lib.sh"
[ -r "$LIB" ] && . "$LIB"

payload=$(cat)

# DOMAINSERV-71: si python3 no está disponible, no podemos parsear → skip
[ -n "$payload" ] && command -v python3 >/dev/null 2>&1 || exit 0

eval "$(printf '%s' "$payload" | python3 -c '
import json, os, re, shlex, sys

try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)

session_id = d.get("session_id", "")
# DOMAINSERV-245: agent_id viene en el payload SOLO cuando el hook se dispara dentro de un
# subagente — la doc de Claude Code lo dice textualmente: "Use this to distinguish subagent
# hook calls from main-thread calls". El session_id, en cambio, SE HEREDA: un subagente usa el
# del padre, y eso es lo que hacía que su corrida acuñara el marker del padre sin dejar rastro.
# Ausente = hilo principal.
agent_id = d.get("agent_id", "") or ""
agent_type = d.get("agent_type", "") or ""
cmd = (d.get("tool_input") or {}).get("command", "") or ""

# ¿El comando es una corrida de tests? (una de las suites soportadas)
# DOMAINSERV-111: las suites en bash (los hooks de install-user se testean con
# `bash x_test.sh`) no se reconocían → el marker nunca se escribía y el
# commit-gate quedaba insatisfacible al tocar justamente estos archivos.
# DOMAINSERV-175: los runners de domain-admin; python\S* cubre python3 y rutas de venv
# DOMAINSERV-254: exigir el sufijo _test.sh dejaba fuera las 11 suites de scripts/tests/,
# que este repo nombra con PREFIJO (test_deploy.sh, test_detect_changed.sh, …). El deny
# las declaraba "de otro tipo" y mandaba al bypass una suite que el hook podía reconocer.
test_re = re.compile(
    r"\b(go\s+test|npm\s+(run\s+)?test|pytest|jest|vitest|cargo\s+test|phpunit|rspec"
    r"|make\s+test|python\S*\s+-m\s+unittest|manage\.py\s+test)\b"
    r"|(?:\bbash\s+|\bsh\s+|\./)(?:\S*/)?(?:test[_-]\S*\.sh|\S*_test\.sh)\b"
)
# --help sale exit 0 y sin FAIL, así que marcaba verde sin correr nada; token
# suelto para no comerse un path con "-h" adentro
ayuda_re = re.compile(r"(?:^|\s)(--help|-h|--version)(?:\s|$)")
is_test = bool(test_re.search(cmd)) and not ayuda_re.search(cmd)

# DOMAINSERV-237: correr `go test` NO es prueba de nada. Un paquete suelto y
# servido del cache dejaba el marker válido para TODO el repo. Para Go se exige la
# suite recursiva (./...) con -count=1 —lo único que garantiza que ningún paquete
# salga del cache— y sin -run, que acota la suite a un subconjunto: `go test
# -count=1 -run TestNada ./...` sale verde sin ejecutar un solo test.
#
# El alcance recorrido viaja en el marker y el gate lo cruza con lo que se va a
# commitear. Eso cierra DOMAINSERV-245: un subagente comparte el session_id del
# padre, así que su corrida escribe el marker del padre — y lo que se le exige NO es
# ser el hilo principal, sino CUBRIR lo que se commitea.
#
# CORRECCIÓN de una premisa que este comentario afirmaba y era falsa: sí hay un campo
# que distingue a un subagente. agent_id viene en el payload cuando el hook se dispara
# dentro de uno, y la doc de Claude Code dice textualmente "use this to distinguish
# subagent hook calls from main-thread calls". Lo que se hereda es el session_id, no
# el agent_id. Se registra en el campo 6 del marker: no cambia la DECISIÓN (que sigue
# siendo por alcance, y así un subagente que corrió la suite correcta puede commitear)
# pero deja auditable quién produjo la evidencia.
#
# Se decide sobre el COMANDO y no sobre el output: el shape del tool_response ya
# rompió este hook dos veces (DOMAINSERV-108), y el comando es dato de entrada.
def base_de(texto, cwd):
    """Directorio donde corre el comando: el cwd del payload más los `cd`.

    El `cd` solo se aplica si el path resultante EXISTE. El cwd del payload es el
    del shell persistente, que puede venir ya posicionado en el destino: ahí
    componer `cwd + cd` da un path doblado (…/services/domain-mcp/services/
    domain-mcp) que no existe, el alcance queda apuntando a la nada y el gate
    deniega un commit legítimo. Pasó de verdad, y un gate que deniega lo legítimo
    empuja al bypass — el modo de falla de DOMAINSERV-111/175/195.
    """
    b = cwd or os.getcwd()
    for seg in re.split(r"&&|;|\n", texto):
        m = re.match(r"\s*cd\s+([^\s;|&]+)", seg)
        if not m:
            continue
        # chr(39)+chr(34) y no los literales: este bloque vive dentro de un string
        # de bash entre comillas simples, y una comilla simple acá lo cierra antes
        # de tiempo — ni siquiera dentro de un comentario
        dst = m.group(1).strip(chr(39) + chr(34))
        cand = dst if dst.startswith("/") else os.path.normpath(os.path.join(b, dst))
        if os.path.isdir(cand):
            b = cand
    return b


def raiz_de_modulo(d):
    """El directorio con go.mod que gobierna d, o "" si no hay ninguno.

    Ancla el alcance a un MÓDULO real en vez del cwd del shell. Sin esto, un cwd que
    resuelve a la raíz del monorepo (donde NO hay go.mod) registraba la raíz entera
    como alcance, y a partir de ahí el chequeo de cobertura quedaba vacuo: cualquier
    archivo del repo caía "dentro". Medido — el alcance acumulado llegó a tener la
    raíz y con eso el guard dejaba pasar todo.
    """
    d = os.path.abspath(d)
    while True:
        if os.path.isfile(os.path.join(d, "go.mod")):
            return d
        padre = os.path.dirname(d)
        if padre == d:
            return ""
        d = padre


def alcance_go(texto, base):
    """Dirs que la corrida recorrió, o [] si la corrida no prueba nada."""
    if re.search(r"(?:^|\s)--?run(?:=|\s)", texto):
        return []
    if not re.search(r"(?:^|\s)--?count[= ]1(?:\s|$)", texto):
        return []
    dirs = []
    for seg in re.split(r"\|\||&&|[;|\n&]", texto):
        if not re.search(r"\bgo\s+(?:-\S+\s+)*test\b", seg):
            continue
        for tok in seg.split():
            m = re.match(r"^(?:\./)?(.*?)/?\.\.\.$", tok)
            if not m:
                continue
            objetivo = os.path.normpath(os.path.join(base, m.group(1) or "."))
            # sin go.mod que lo gobierne no hubo corrida de Go que valga: registrar
            # ese path como alcance es afirmar una cobertura que nadie ejecutó
            if raiz_de_modulo(objetivo):
                dirs.append(objetivo)
    return dirs


es_go = is_test and bool(re.search(r"\bgo\s+(?:-\S+\s+)*test\b", cmd))
if es_go:
    scopes_abs = alcance_go(cmd, base_de(cmd, d.get("cwd", "")))
    es_prueba = bool(scopes_abs)
else:
    # los demás runners conservan su semántica previa: alcance = repo entero
    scopes_abs = []
    es_prueba = is_test

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
    r"(?m)^[ \t]*FAIL\b",                    # go: línea FAIL\tpkg · shell: "  FAIL: label"
    r"--- FAIL:",                            # go: subtests
    r"\bpanic:\s",                           # go: panic
    r"test result:\s*FAILED",                # cargo test
    r"FAILURES!",                            # phpunit
    r"(?im)\b[1-9]\d*\s+failed\b",           # jest/vitest/pytest: "N failed"
    r"(?im)\b[1-9]\d*\s+failing\b",          # mocha: "N failing"
    r"(?im)\b[1-9]\d*\s+failure(s)?\b",      # rspec: "N failures"
    # unittest: ^FAIL\b no matchea FAILED porque la E rompe el \b (DOMAINSERV-175)
    r"FAILED\s*\([^)]*(?:failures|errors)=[1-9]",
    # DOMAINSERV-254: las suites de shell de este repo reportan en español. Reconocer su
    # runner sin reconocer su rojo sería peor que no reconocerlo: `ok = is_test and not
    # signal_fail` (abajo) acuñaría el marker tests-ok sobre una suite ROJA, porque el
    # tool_response de Bash no expone exit_code.
    r"(?m)^[ \t]*FALLA\b",                   # test_install_normaliza_duenos.sh
    # "3 fallaron" / "3 asserts fallaron" / "3 de 12 tests fallaron". El [1-9] deja pasar
    # el "AMARILLO — 0 fallaron" de test_auto_deploy_entorno.sh, que es verde.
    r"(?im)\b[1-9]\d*(?:\s+\S+){0,3}\s+fallaron\b",
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
# el origen viaja al marker como campo 6: no cambia QUIÉN puede commitear —eso lo decide el
# alcance de DOMAINSERV-237— pero deja auditable quién produjo la evidencia
print("agent_id=%s" % shlex.quote(agent_id))
print("agent_type=%s" % shlex.quote(agent_type))
print("is_test=%s" % shlex.quote("1" if is_test else "0"))
print("tests_ok=%s" % shlex.quote("1" if ok else "0"))
print("es_prueba=%s" % shlex.quote("1" if es_prueba else "0"))
print("scopes=%s" % shlex.quote(" ".join(scopes_abs)))
# el runner queda registrado porque decide la semántica del alcance: "go" exige
# cobertura por path, y cualquier otro conserva el alcance histórico (repo entero)
print("runner=%s" % shlex.quote("go" if es_go else "otro"))
' 2>/dev/null)"

# Sin session_id o comando que no es test → no-op.
[ -n "$session_id" ] || exit 0
[ "$is_test" = "1" ] || exit 0

state_dir="$HOME/.local/state/domain"
mkdir -p "$state_dir" 2>/dev/null
marker="$state_dir/tests-ok-$session_id"

if [ "$tests_ok" = "1" ]; then
  # DOMAINSERV-237: un verde que NO prueba nada es "no es evidencia", no "es un
  # fallo". Si borrara el marker, correr `go test ./unpaquete` después de la suite
  # completa dejaría el gate cerrado sin forma de reabrirlo salvo el bypass — el
  # modo de falla de DOMAINSERV-111/175/195. Así que es un no-op.
  [ "$es_prueba" = "1" ] || exit 0

  # DOMAINSERV-74: marker con timestamp + tree hash (git diff HEAD) para
  # invalidar ante cualquier edición posterior.
  # DOMAINSERV-266: domain_sha256 y no sha256sum directo — en macOS la utilidad es
  # `shasum -a 256`, y un hash vacío acá deja el marker sin alcance y el gate cerrado.
  tree_hash=$(git diff --no-color HEAD 2>/dev/null | domain_sha256)
  # DOMAINSERV-219: field3 = hash del contenido del código, insensible a los
  # archivos inertes y a que HEAD se mueva. field2 queda por compatibilidad.
  code_hash=""
  type domain_tests_code_hash >/dev/null 2>&1 && code_hash=$(domain_tests_code_hash)

  # DOMAINSERV-237: field4 = alcance recorrido. Se ACUMULA entre corridas del mismo
  # árbol, porque un commit puede tocar dos módulos y eso son dos corridas; si el
  # code_hash cambió, el alcance viejo ya no aplica y se descarta.
  scopes_previos=""
  if [ -r "$marker" ]; then
    hash_previo=$(cut -f3 "$marker" 2>/dev/null | head -1)
    if [ -n "$code_hash" ] && [ "$hash_previo" = "$code_hash" ]; then
      scopes_previos=$(cut -f4 "$marker" 2>/dev/null | head -1)
    fi
  fi
  scopes_acumulados=$(printf '%s %s' "$scopes_previos" "$scopes" | tr ' ' '\n' | grep -v '^$' | sort -u | tr '\n' ' ' | sed 's/ $//')

  # DOMAINSERV-245: field6 = el ORIGEN de la corrida. "main" cuando la hizo el hilo principal,
  # o "<agent_type>:<agent_id>" cuando la hizo un subagente. Es un campo NUEVO al final a
  # propósito: los lectores viejos usan cut -f1..f5 y no se enteran, y un marker escrito antes
  # de este cambio devuelve vacío en f6, que se lee como origen desconocido.
  #
  # NO cambia quién puede commitear: eso lo decide el alcance (field4, DOMAINSERV-237). Lo que
  # agrega es que el deny pueda decir QUIÉN produjo la evidencia — sin esto, un marker acuñado
  # por un subagente era indistinguible de uno del hilo principal.
  origen="main"
  if [ -n "${agent_id:-}" ]; then
    origen="${agent_type:-subagente}:${agent_id}"
  fi
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$(date -Iseconds)" "${tree_hash:-}" "${code_hash:-}" "${scopes_acumulados:-}" "${runner:-otro}" "$origen" > "$marker" 2>/dev/null
else
  rm -f "$marker" 2>/dev/null
fi
exit 0
