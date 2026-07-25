#!/usr/bin/env bash
# DOMAINSERV-131 fase 1: el fingerprint de escaneo registra METADATOS del ataque
# (metodo, content-type, longitud declarada) y NUNCA el contenido del body.
#
# Corre Caddy real (caddy:2-alpine) contra el Caddyfile del repo, con limite de
# memoria, y verifica 4 invariantes + 2 controles negativos (sabotaje).
#
# Uso: ./caddyfile-scan-fingerprint_test.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CADDYFILE="$SCRIPT_DIR/Caddyfile"
IMAGE="caddy:2-alpine"
PORT="${CADDY_TEST_PORT:-18131}"
NET="scanfp-net-$$"
CADDY_NAME="scanfp-caddy-$$"
STUB_NAME="scanfp-stub-$$"
MEM_LIMIT="256m"
# canary que viaja SOLO en el body: si aparece en el log, se archivo contenido
CANARY="pwd-CANARY-31f4c2a9"
CANARY_B64="$(printf '%s' "$CANARY" | base64)"
BASE="http://127.0.0.1:$PORT"
WORK="$(mktemp -d)"
FAILURES=0

log()  { printf '%s\n' "$*"; }
pass() { printf '  PASS  %s\n' "$*"; }
fail() { printf '  FAIL  %s\n' "$*"; FAILURES=$((FAILURES + 1)); }

cleanup() {
  docker rm -f "$CADDY_NAME" "$STUB_NAME" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

# el bloque de opciones globales tiene que ser el PRIMER bloque: se antepone
# aca (nunca en el repo) para que el test no dispare ACME contra quiensabe.cl
prepare_config() {
  local sabotage="$1" out="$WORK/Caddyfile.$1"
  printf '{\n  auto_https off\n}\n' >"$out"
  case "$sabotage" in
    none) cat "$CADDYFILE" >>"$out" ;;
    # sabotaje 1: quita el prefijo `<` -> el append pasa a late y abort lo borra
    late) sed 's/log_append <scan_/log_append scan_/' "$CADDYFILE" >>"$out" ;;
    # sabotaje 2: archiva el body (lo que la fase 1 original pedia y viola la policy)
    body)
      sed '/log_append <scan_content_length/a\    log_append <scan_body {http.request.body}' \
        "$CADDYFILE" >>"$out"
      ;;
    *) log "modo de sabotaje desconocido: $sabotage"; exit 2 ;;
  esac
  printf '%s' "$out"
}

write_stub_config() {
  cat >"$WORK/stub.Caddyfile" <<'EOF'
{
  auto_https off
  admin off
}
:8000 {
  respond "stub-ok" 200
}
EOF
}

# el access log sale por stdout del container: se captura redirigiendo el
# `docker run` en foreground (no `docker logs`, que reformatea la salida)
start_stack() {
  local cfg="$1" out="$2"
  docker network create "$NET" >/dev/null
  docker run --rm -d --name "$STUB_NAME" --network "$NET" --network-alias domain-mcp \
    -v "$WORK/stub.Caddyfile:/etc/caddy/Caddyfile:ro" "$IMAGE" >/dev/null
  docker run --rm --name "$CADDY_NAME" --network "$NET" \
    --memory "$MEM_LIMIT" --memory-swap "$MEM_LIMIT" -p "$PORT:80" \
    -v "$cfg:/etc/caddy/Caddyfile:ro" "$IMAGE" >"$out" 2>&1 &
  local i
  for i in $(seq 1 40); do
    if curl -s -o /dev/null -m 2 "$BASE/mcp"; then return 0; fi
    sleep 0.25
  done
  log "Caddy no llego a servir en $BASE"
  tail -20 "$out"
  return 1
}

stop_stack() {
  docker rm -f "$CADDY_NAME" "$STUB_NAME" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  # el redirect necesita un instante para vaciar el buffer
  sleep 1
  wait 2>/dev/null || true
}

send_scan_probes() {
  curl -s -o /dev/null -m 5 -X POST "$BASE/.env" \
    -H 'Content-Type: application/x-www-form-urlencoded' -d "password=$CANARY" || true
  curl -s -o /dev/null -m 5 -X POST "$BASE/wp-login.php" \
    -H 'Content-Type: application/json' -d "{\"pwd\":\"$CANARY\"}" || true
  curl -s -o /dev/null -m 5 -X POST "$BASE/xmlrpc.php" \
    -H 'Content-Type: application/octet-stream' --data-binary "$CANARY_B64" || true
  curl -s -o /dev/null -m 5 -X POST "$BASE/admin.php" -F "password=$CANARY" || true
  curl -s -o /dev/null -m 5 "$BASE/.git/config" || true
}

send_legit_probes() {
  head -c 5000 /dev/zero | tr '\0' 'x' >"$WORK/body5k.txt"
  curl -s -o /dev/null -m 10 -X POST "$BASE/api/probe" \
    -H 'Content-Type: text/plain' --data-binary "@$WORK/body5k.txt"
  curl -s -o /dev/null -m 10 "$BASE/mcp"
}

# reproduce el POST de 300MB que mato a Caddy (exit 137) cuando la fase 1
# logeaba el body: en chunked, para no bufferizar 300MB en el runner
send_flood_probe() {
  head -c 314572800 /dev/zero | curl -s -o /dev/null -m 30 -X POST -T - "$BASE/.env" || true
}

assert_container_alive() {
  local state
  state="$(docker inspect -f '{{.State.Status}}:{{.State.ExitCode}}' "$CADDY_NAME" 2>/dev/null || echo missing)"
  if [ "$state" != "running:0" ]; then
    fail "Caddy no sobrevivio al POST de 300MB (state=$state, 137=OOM)"
    return 1
  fi
  if ! curl -s -o /dev/null -m 5 "$BASE/mcp"; then
    fail "Caddy sigue vivo pero dejo de servir despues del flood"
    return 1
  fi
  pass "Caddy sobrevive al POST de 300MB a /.env y sigue sirviendo"
}

assert_fingerprint_on_scanners() { python3 "$WORK/verify.py" scanners "$1"; }
assert_legit_untouched()         { python3 "$WORK/verify.py" legit "$1"; }
assert_no_body_content()         { python3 "$WORK/verify.py" nobody "$1" "$CANARY" "$CANARY_B64"; }

write_verifier() {
  cat >"$WORK/verify.py" <<'PY'
import json, sys

MODE, LOG = sys.argv[1], sys.argv[2]
FP_KEYS = ("scan_method", "scan_content_type", "scan_content_length")
# uri -> (metodo, prefijo de content-type esperado)
SCAN_SPEC = {
    "/.env": ("POST", "application/x-www-form-urlencoded"),
    "/wp-login.php": ("POST", "application/json"),
    "/xmlrpc.php": ("POST", "application/octet-stream"),
    "/admin.php": ("POST", "multipart/form-data"),
    "/.git/config": ("GET", ""),
}
LEGIT = ("/api/probe", "/mcp")


def entries(path):
    out = []
    for line in open(path, encoding="utf-8", errors="replace"):
        line = line.strip()
        if '"msg":"handled request"' not in line:
            continue
        try:
            out.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return out


def check_scanners(rows):
    errs = []
    seen = {}
    for e in rows:
        uri = e["request"]["uri"]
        if uri in SCAN_SPEC:
            seen.setdefault(uri, []).append(e)
    for uri, (method, ct) in SCAN_SPEC.items():
        got = seen.get(uri)
        if not got:
            errs.append(f"{uri}: sin entry en el log")
            continue
        e = got[0]
        missing = [k for k in FP_KEYS if k not in e]
        if missing:
            errs.append(f"{uri}: faltan campos {missing} (status={e.get('status')})")
            continue
        if e.get("status") != 0:
            errs.append(f"{uri}: status={e.get('status')}, se esperaba 0 (abortado)")
        if e["scan_method"] != method:
            errs.append(f"{uri}: scan_method={e['scan_method']!r} != {method!r}")
        if not e["scan_content_type"].startswith(ct):
            errs.append(f"{uri}: scan_content_type={e['scan_content_type']!r} no empieza con {ct!r}")
        if method == "POST" and not e["scan_content_length"].isdigit():
            errs.append(f"{uri}: scan_content_length={e['scan_content_length']!r} no es numero")
    return errs


def check_legit(rows):
    errs = []
    for uri in LEGIT:
        got = [e for e in rows if e["request"]["uri"] == uri]
        if not got:
            errs.append(f"{uri}: sin entry en el log")
            continue
        e = got[-1]
        leaked = [k for k in e if k.startswith("scan_")]
        if leaked:
            errs.append(f"{uri}: ruta legitima con campos de fingerprint {leaked}")
        if e.get("status") != 200:
            errs.append(f"{uri}: status={e.get('status')}, se esperaba 200 del upstream")
    body = [e for e in rows if e["request"]["uri"] == "/api/probe"]
    if body and body[-1].get("bytes_read") != 5000:
        errs.append(f"/api/probe: bytes_read={body[-1].get('bytes_read')} != 5000 (body alterado)")
    return errs


def check_nobody(path):
    raw = open(path, encoding="utf-8", errors="replace").read()
    return [f"el log contiene {n!r} del body" for n in sys.argv[3:] if n in raw]


rows = entries(LOG)
if MODE == "scanners":
    problems = check_scanners(rows)
elif MODE == "legit":
    problems = check_legit(rows)
else:
    problems = check_nobody(LOG)
for p in problems:
    print("    -", p)
sys.exit(1 if problems else 0)
PY
}

run_baseline() {
  local cfg
  cfg="$(prepare_config none)"
  start_stack "$cfg" "$WORK/baseline.log"
  send_scan_probes
  send_legit_probes
  send_flood_probe
  assert_container_alive || true
  stop_stack
  log "[1] fingerprint en rutas de escaneo (incluidas las abortadas)"
  assert_fingerprint_on_scanners "$WORK/baseline.log" \
    && pass "metodo + content-type + longitud presentes en las 5 rutas, status=0" \
    || fail "el fingerprint no llega a las rutas de escaneo"
  log "[2] rutas legitimas (/api/, /mcp) intactas"
  assert_legit_untouched "$WORK/baseline.log" \
    && pass "sin campos scan_*, status 200 y body de 5000 bytes intacto" \
    || fail "las rutas legitimas quedaron afectadas"
  log "[3] policy secrets-redaction: nada del body en el log"
  assert_no_body_content "$WORK/baseline.log" \
    && pass "canary ausente en las 4 codificaciones (urlencoded, json, base64, multipart)" \
    || fail "se archivo contenido del body"
}

# control negativo: sin el `<` el append es late y abort lo borra; si la
# asercion [1] igual pasa, la asercion no sirve
run_sabotage_late() {
  local cfg
  cfg="$(prepare_config late)"
  start_stack "$cfg" "$WORK/sabotage-late.log"
  send_scan_probes
  stop_stack
  log "[4] sabotaje: log_append sin el prefijo '<'"
  if assert_fingerprint_on_scanners "$WORK/sabotage-late.log" >/dev/null 2>&1; then
    fail "la asercion [1] paso con el fingerprint saboteado: no detecta nada"
  else
    pass "la asercion [1] detecta la perdida del fingerprint en rutas abortadas"
  fi
}

# control negativo: si alguien vuelve a logear el body, la asercion [3] lo caza
run_sabotage_body() {
  local cfg
  cfg="$(prepare_config body)"
  start_stack "$cfg" "$WORK/sabotage-body.log"
  send_scan_probes
  stop_stack
  log "[5] sabotaje: log_append del body ({http.request.body})"
  if assert_no_body_content "$WORK/sabotage-body.log" >/dev/null 2>&1; then
    fail "la asercion [3] paso con el body archivado: no protege la policy"
  else
    pass "la asercion [3] detecta el body archivado"
  fi
}

main() {
  command -v docker >/dev/null || { log "docker no disponible"; exit 2; }
  [ -f "$CADDYFILE" ] || { log "no existe $CADDYFILE"; exit 2; }
  write_stub_config
  write_verifier
  log "== DOMAINSERV-131 fase 1: fingerprint de escaneo =="
  run_baseline
  run_sabotage_late
  run_sabotage_body
  log "=================================================="
  if [ "$FAILURES" -ne 0 ]; then
    log "RESULTADO: $FAILURES asercion(es) fallaron"
    exit 1
  fi
  log "RESULTADO: todas las aserciones pasaron"
}

main "$@"
