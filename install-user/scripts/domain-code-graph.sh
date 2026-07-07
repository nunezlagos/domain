#!/usr/bin/env bash
# scripts/domain-code-graph.sh
#
# Construye el code graph de un repo LOCAL y lo sube al server domain-mcp.
# Reemplaza a domain_code_build server-side: domain-mcp corre en el VPS y no
# tiene acceso al filesystem del cliente, asi que el parseo se hace aca con
# ast-grep (sg) y se sube via la tool MCP domain_code_upload.
#
# ENFOQUE LITE (2026-07-05): solo SYMBOLS, sin edges.
# Los edges "calls" heuristicos (regex nombre+parentesis, resolucion por nombre
# global) producian god-nodes falsos (todo `New(...)` atribuido a un solo nodo)
# y eran el 90% del costo: 50K SELECTs server-side por upload (~20 min de
# transaccion, lock storms 55P03 entre uploads concurrentes). El outline de
# simbolos por archivo es correcto, barato y suficiente para navegacion.
#
# INCREMENTAL: guarda el ultimo git head subido en
# ~/.local/state/domain/code-graph-head-<slug>. En corridas siguientes solo
# parsea y sube los archivos cambiados desde ese head (commits + working tree
# + untracked). El server reemplaza nodos por archivo (SoftDeleteNodesByFileExcept),
# asi que el re-upload parcial no duplica ni acumula basura.
# Limitacion conocida: los nodos de archivos BORRADOS del repo quedan en el
# grafo hasta el proximo full scan (el UploadInput del server no acepta
# deleted_files todavia).
#
# Multi-lenguaje via ast-grep: TypeScript/JavaScript/TSX/JSX/Go/Python/Rust/Java.
#
# Uso:
#   domain-code-graph.sh [REPO_PATH] [PROJECT_SLUG]
#   - REPO_PATH: path absoluto al repo (default: cwd)
#   - PROJECT_SLUG: slug del project en domain (default: basename del repo)
#
# GUARDAS DE MEMORIA (2026-07-07): 4 capas anti-crash tras incidente OOM
# (N sesiones concurrentes x JSON completo en RAM congelaron la maquina):
#   capa 1: flock singleton por maquina (builds concurrentes salen solos)
#   capa 2: ast-grep --json=stream + parseo linea a linea (memoria O(1))
#   capa 3: cgroup v2 MemoryMax via systemd-run (techo duro, default 1G;
#           fallback ulimit -v si no hay systemd)
#   capa 4: oom_score_adj=900 (victima preferente) + pre-flight MemAvailable
#
# Env vars:
#   DOMAIN_CODE_GRAPH_FULL=1              fuerza full scan (ignora el estado incremental)
#   DOMAIN_CODE_GRAPH_MAX_FILE_BYTES=N    saltear archivos mayores a N bytes (default 2 MB)
#   DOMAIN_CODE_GRAPH_MEM_MAX=N           techo de RAM del cgroup (default 1G)
#   DOMAIN_CODE_GRAPH_MIN_AVAIL_MB=N      minimo MemAvailable para arrancar (default 2048)
#   DOMAIN_VPS_URL / DOMAIN_API_KEY       override de la config del installer
#
# Requiere: ast-grep, curl, python3, git (para el modo incremental).

set -uo pipefail

# NO usamos `set -e` porque ast-grep puede retornar !=0 para archivos que no
# matchean patterns, y eso mataría el script entero. En su lugar, manejamos
# errores explicitamente con `|| true` donde corresponde.

REPO_PATH="${1:-$(pwd)}"
PROJECT_SLUG="${2:-$(basename "$REPO_PATH")}"

# ---------- 0. guardas de memoria (4 capas anti-crash, 2026-07-07) ----------
# El build sin limites tumbo la maquina del cliente: N sesiones concurrentes
# construyendo el grafo a la vez, cada una cargando el JSON completo de
# ast-grep en RAM. Capas: cgroup MemoryMax > flock singleton > oom_score_adj
# + pre-flight MemAvailable > parseo streaming (en extract_for_lang).

# Capa 3 — techo duro de RAM via cgroup v2 (systemd-run). Si el scope supera
# MEM_MAX el kernel mata SOLO este arbol de procesos, nunca el resto del sistema.
MEM_MAX="${DOMAIN_CODE_GRAPH_MEM_MAX:-1G}"
if [[ -z "${DOMAIN_CG_WRAPPED:-}" ]] && command -v systemd-run >/dev/null 2>&1 && systemd-run --user --scope --quiet -p MemoryMax=infinity true 2>/dev/null; then
  exec env DOMAIN_CG_WRAPPED=1 systemd-run --user --scope --quiet \
    --nice=19 \
    -p MemoryMax="$MEM_MAX" -p MemorySwapMax=256M \
    "$0" "$@"
fi
# Fallback sin systemd: limite de memoria virtual por proceso (malloc falla ->
# el proceso muere con error en vez de congelar la maquina).
if [[ -z "${DOMAIN_CG_WRAPPED:-}" ]]; then
  ulimit -v 1500000 2>/dev/null || true
fi

# Capa 1 — singleton por maquina: si otra sesion ya esta construyendo un
# grafo, salir silenciosamente (el estado incremental cubre lo pendiente).
GRAPH_LOCK="${XDG_RUNTIME_DIR:-/tmp}/domain-code-graph.lock"
exec 9>"$GRAPH_LOCK"
if ! flock -n 9; then
  echo "[lock] otro build de code graph en curso — salgo (lo pendiente se toma en la proxima corrida)"
  exit 0
fi

# Capa 4a — victima preferente del OOM killer: si igual hay presion de
# memoria, el kernel mata este script antes que cualquier app del usuario.
echo 900 > /proc/self/oom_score_adj 2>/dev/null || true

# Capa 4b — backpressure: no arrancar si la maquina ya esta corta de memoria.
MIN_AVAIL_MB="${DOMAIN_CODE_GRAPH_MIN_AVAIL_MB:-2048}"
avail_mb=$(awk '/MemAvailable/{printf "%d", $2/1024}' /proc/meminfo 2>/dev/null || echo 999999)
if (( avail_mb < MIN_AVAIL_MB )); then
  echo "[mem] solo ${avail_mb}MB disponibles (<${MIN_AVAIL_MB}MB) — difiero el build para no estresar la maquina"
  exit 0
fi

# ---------- 1. resolver VPS URL + API key (mismo orden que el hook de session) ----------
VPS_URL="${DOMAIN_VPS_URL:-}"
API_KEY="${DOMAIN_API_KEY:-}"
if [ -z "$VPS_URL" ] || [ -z "$API_KEY" ]; then
  for envf in "$HOME/.config/domain/install.env" "$HOME/.claude/.env" "$HOME/.config/opencode/.env"; do
    [ -r "$envf" ] || continue
    while IFS='=' read -r k v; do
      v="${v%\"}"; v="${v#\"}"; v="${v%\'}"; v="${v#\'}"
      case "${k#DOMAIN_}" in
        VPS_URL)          [ -z "$VPS_URL" ] && VPS_URL="$v" ;;
        MCP_API_KEY|API_KEY) [ -z "$API_KEY" ] && API_KEY="$v" ;;
      esac
    done < "$envf"
  done
fi
# fallback: extraer del opencode.json si no se encontro arriba
if [ -z "$VPS_URL" ] || [ -z "$API_KEY" ]; then
  for jsonf in "$HOME/.config/opencode/opencode.json" "$HOME/.claude.json"; do
    [ -r "$jsonf" ] || continue
    extracted=$(python3 -c "
import json
try:
  d=json.load(open('$jsonf'))
  for cont in ('mcp','mcpServers'):
    e=d.get(cont,{}).get('domain-mcp',{})
    u=e.get('url','')
    h=e.get('headers',{}).get('Authorization','')
    if u and h.startswith('Bearer '):
      print(u.rstrip('/mcp').rstrip('/'), h[7:])
      break
except: pass
" 2>/dev/null)
    [ -n "$extracted" ] && { VPS_URL=$(echo "$extracted" | awk '{print $1}'); API_KEY=$(echo "$extracted" | awk '{print $2}'); break; }
  done
fi
if [ -z "$VPS_URL" ] || [ -z "$API_KEY" ]; then
  echo "ERROR: no pude resolver VPS_URL o API_KEY. Configuralos en ~/.config/domain/install.env, ~/.claude/.env o ~/.config/opencode/.env" >&2
  exit 1
fi

# ---------- 2. asegurar ast-grep instalado (sin sudo, lo asume el installer) ----------
if ! command -v ast-grep >/dev/null 2>&1; then
  echo "ERROR: ast-grep no esta instalado en este sistema." >&2
  echo "" >&2
  echo "ast-grep es una dependencia del code graph. Lo instala el installer del user:" >&2
  echo "  curl -fsSL https://raw.githubusercontent.com/nunezlagos/domain/main/install-user/install-curl.sh | sudo bash" >&2
  echo "" >&2
  echo "(el install detecta tu distro e instala ast-grep via pacman/apt/brew/cargo automaticamente)" >&2
  exit 1
fi
echo "[scan] ast-grep OK: $(ast-grep --version 2>&1 | head -1)"

# ---------- 3. git head best-effort ----------
GIT_HEAD=$(git -C "$REPO_PATH" rev-parse HEAD 2>/dev/null || echo "")

# ---------- 4. modo incremental: solo archivos cambiados desde el ultimo upload ----------
CODE_EXT_RE='\.(go|ts|tsx|js|jsx|py|rs|java)$'
EXCLUDE_RE='(^|/)(node_modules|dist|build|\.git|vendor)/'
STATE_DIR="$HOME/.local/state/domain"
STATE_FILE="$STATE_DIR/code-graph-head-$PROJECT_SLUG"
MODE="full"
LAST_HEAD=""
if [ "${DOMAIN_CODE_GRAPH_FULL:-0}" != "1" ] && [ -n "$GIT_HEAD" ] && [ -r "$STATE_FILE" ]; then
  LAST_HEAD=$(tr -d '[:space:]' < "$STATE_FILE")
  if [ -n "$LAST_HEAD" ] && git -C "$REPO_PATH" cat-file -e "$LAST_HEAD" 2>/dev/null; then
    MODE="incremental"
  fi
fi

if [ "$MODE" = "incremental" ]; then
  # commits desde el ultimo upload + working tree (staged/unstaged) + untracked
  mapfile -t FILES < <(
    {
      git -C "$REPO_PATH" diff --name-only --diff-filter=ACMR "$LAST_HEAD" HEAD 2>/dev/null
      git -C "$REPO_PATH" diff --name-only --diff-filter=ACMR HEAD 2>/dev/null
      git -C "$REPO_PATH" ls-files --others --exclude-standard 2>/dev/null
    } | sort -u | grep -E "$CODE_EXT_RE" | grep -vE "$EXCLUDE_RE" | sed "s|^|$REPO_PATH/|"
  )
  if [ ${#FILES[@]} -eq 0 ]; then
    echo "[scan] incremental: sin cambios de codigo desde $LAST_HEAD — nada que indexar"
    exit 0
  fi
  echo "[scan] incremental desde ${LAST_HEAD:0:12}: ${#FILES[@]} archivos cambiados"
else
  mapfile -t FILES < <(find "$REPO_PATH" -type f \
    \( -name "*.go" -o -name "*.ts" -o -name "*.tsx" -o -name "*.js" -o -name "*.jsx" -o -name "*.py" -o -name "*.rs" -o -name "*.java" \) \
    -not -path "*/node_modules/*" -not -path "*/dist/*" -not -path "*/build/*" -not -path "*/.git/*" -not -path "*/vendor/*" \
    2>/dev/null)
  echo "[scan] full scan: ${#FILES[@]} archivos candidatos"
fi

# Filtrar archivos generados (sqlc/protobuf/etc) y demasiado grandes: no aportan
# al grafo y consumen parseo. Configurable: DOMAIN_CODE_GRAPH_MAX_FILE_BYTES (default 2 MB).
if [ ${#FILES[@]} -gt 0 ]; then
  mapfile -d '' -t FILES < <(printf '%s\0' "${FILES[@]}" | python3 -c "
import sys, os
max_bytes = int(os.environ.get('DOMAIN_CODE_GRAPH_MAX_FILE_BYTES', '2097152'))
markers = ('Code generated', 'DO NOT EDIT', '@generated')
raw = sys.stdin.buffer.read().rstrip(b'\x00')
paths = raw.split(b'\x00') if raw else []
skipped_size = skipped_gen = 0
for p in paths:
    fp = p.decode('utf-8', errors='replace')
    try:
        if os.path.getsize(fp) > max_bytes:
            skipped_size += 1
            continue
        with open(fp, 'r', encoding='utf-8', errors='ignore') as fh:
            head = ''.join(fh.readline() for _ in range(5))
    except OSError:
        continue
    if any(m in head for m in markers):
        skipped_gen += 1
        continue
    sys.stdout.buffer.write(p + b'\x00')
if skipped_size or skipped_gen:
    print(f'[scan] salteados: {skipped_gen} generados, {skipped_size} archivos > {max_bytes} bytes', file=sys.stderr)
")
fi

if [ ${#FILES[@]} -eq 0 ]; then
  echo "[scan] no encontre archivos a parsear en $REPO_PATH"
  exit 0
fi
echo "[scan] ${#FILES[@]} archivos a parsear"

# ---------- 5. extraer nodos con ast-grep ----------
# Solo corre patterns para los lenguajes realmente presentes (ahorra ~70%
# de invocaciones). Detecta extensión desde FILES.
NODES_FILE=$(mktemp)
GRAPH_FILE=$(mktemp)
REQ_FILE=$(mktemp)
trap 'rm -f "$NODES_FILE" "$GRAPH_FILE" "$REQ_FILE"' EXIT

declare -A LANG_EXT
LANG_EXT[typescript]=".ts .tsx"
LANG_EXT[javascript]=".js .jsx .mjs .cjs"
LANG_EXT[go]=".go"
LANG_EXT[python]=".py"
LANG_EXT[rust]=".rs .rlib"
LANG_EXT[java]=".java"
LANG_EXT[tsx]=".tsx"
declare -A HAS_LANG
for f in "${FILES[@]}"; do
  ext="${f##*.}"
  for lang in "${!LANG_EXT[@]}"; do
    for e in ${LANG_EXT[$lang]}; do
      [[ ".$ext" == "$e" ]] && HAS_LANG[$lang]=1
    done
  done
done

extract_for_lang() {
  local lang="$1" pattern="$2" kind="$3"
  [[ -v HAS_LANG[$lang] && "${HAS_LANG[$lang]}" == "1" ]] || return
  # Capa 2 — streaming: --json=stream emite UN objeto JSON por linea; el
  # parser procesa e imprime linea a linea. Memoria O(1) por match, nunca
  # el repo completo en RAM (el slurp con sys.stdin.read() causaba OOM).
  printf '%s\0' "${FILES[@]}" | xargs -0 -n 64 ast-grep run \
    --pattern "$pattern" --lang "$lang" --json=stream 2>/dev/null | \
    python3 -c "
import sys, json
pat_kind = '$kind'
prefix = '$REPO_PATH/'
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        m = json.loads(line)
    except json.JSONDecodeError:
        continue
    file_path = m.get('file','')
    rel = file_path[len(prefix):] if file_path.startswith(prefix) else file_path
    text = m.get('text','').split('\n')[0]
    line_start = m.get('range',{}).get('start',{}).get('line', 0) + 1
    line_end = m.get('range',{}).get('end',{}).get('line', line_start)
    name = text.split('(')[0].split('{')[0].strip()
    for kw in ('class ', 'function ', 'func ', 'def ', 'fn ', 'export function ', 'export const ', 'export default function ', 'async function ', 'public function ', 'private function '):
        if name.startswith(kw):
            name = name[len(kw):].strip()
            break
    qn = f'{rel}:{line_start}:{name}' if name else f'{rel}:{line_start}'
    print(f'{rel}\t{pat_kind}\t{name}\t{qn}\t{line_start}\t{line_end}\t{text}')
" 2>/dev/null
}

# Patterns por lenguaje — solo los que tengan archivos en el repo
extract_for_lang typescript 'export function $FN($$$) { $$$ }' function >> "$NODES_FILE"
extract_for_lang typescript 'function $FN($$$) { $$$ }' function >> "$NODES_FILE"
extract_for_lang typescript 'export const $FN = ($$$) => $$$' function >> "$NODES_FILE"
extract_for_lang typescript 'class $CLS { $$$ }' type >> "$NODES_FILE"
extract_for_lang typescript 'interface $IF { $$$ }' interface >> "$NODES_FILE"
extract_for_lang typescript 'type $T = $$$' type >> "$NODES_FILE"
extract_for_lang tsx 'export function $FN($$$) { $$$ }' function >> "$NODES_FILE"
extract_for_lang tsx 'function $FN($$$) { $$$ }' function >> "$NODES_FILE"
extract_for_lang tsx 'const $FN: $T = ($$$) => $$$' function >> "$NODES_FILE"
extract_for_lang tsx 'class $CLS extends $$$ { $$$ }' type >> "$NODES_FILE"
extract_for_lang javascript 'function $FN($$$) { $$$ }' function >> "$NODES_FILE"
extract_for_lang javascript 'export function $FN($$$) { $$$ }' function >> "$NODES_FILE"
extract_for_lang javascript 'const $FN = ($$$) => $$$' function >> "$NODES_FILE"
extract_for_lang javascript 'class $CLS { $$$ }' type >> "$NODES_FILE"
extract_for_lang go 'func $FN($$$) $$$ { $$$ }' function >> "$NODES_FILE"
extract_for_lang go 'func ($$$) $FN($$$) $$$ { $$$ }' method >> "$NODES_FILE"
extract_for_lang go 'type $T struct { $$$ }' type >> "$NODES_FILE"
extract_for_lang go 'type $T interface { $$$ }' interface >> "$NODES_FILE"
extract_for_lang python 'def $FN($$$): $$$' function >> "$NODES_FILE"
extract_for_lang python 'class $CLS: $$$' type >> "$NODES_FILE"
extract_for_lang python 'async def $FN($$$): $$$' function >> "$NODES_FILE"
extract_for_lang rust 'fn $FN($$$) $$$ { $$$ }' function >> "$NODES_FILE"
extract_for_lang rust 'pub fn $FN($$$) $$$ { $$$ }' function >> "$NODES_FILE"
extract_for_lang rust 'struct $S { $$$ }' type >> "$NODES_FILE"
extract_for_lang rust 'impl $$$ { $$$ }' type >> "$NODES_FILE"
extract_for_lang rust 'trait $T { $$$ }' interface >> "$NODES_FILE"
extract_for_lang java 'public $T $FN($$$) { $$$ }' method >> "$NODES_FILE"
extract_for_lang java 'private $T $FN($$$) { $$$ }' method >> "$NODES_FILE"
extract_for_lang java 'class $CLS { $$$ }' type >> "$NODES_FILE"

NODES_COUNT=$(wc -l < "$NODES_FILE" || echo 0)
echo "[parse] $NODES_COUNT nodos detectados (symbols-only, sin edges)"

# ---------- 6. construir el JSON del grafo (vía python, robusto) ----------
FILE_COUNT=${#FILES[@]}
# Escribir a archivo temporal para evitar "argument list too long"
python3 -c "
import json,sys
nodes=[]
seen=set()
with open('$NODES_FILE') as f:
    for line in f:
        parts=line.rstrip('\n').split('\t')
        if len(parts)<7: continue
        rel,kind,name,qn,ls,le,text=parts
        if qn in seen: continue
        seen.add(qn)
        nodes.append({'kind':kind,'name':name,'qualified_name':qn,'file_path':rel,'line_start':int(ls) if ls else 0,'line_end':int(le) if le else 0,'signature':text[:200],'doc':''})
# El tool espera project_slug + graph_json:{files_scanned,git_head,nodes,edges}
# edges siempre vacio: enfoque symbols-only (ver header).
with open('$GRAPH_FILE','w') as f:
    json.dump({
        'project_slug':'$PROJECT_SLUG',
        'graph_json': {
            'git_head':'$GIT_HEAD',
            'files_scanned':$FILE_COUNT,
            'nodes':nodes,
            'edges':[],
        }
    }, f)
"

echo "[upload] $NODES_COUNT nodos de $FILE_COUNT archivos ($MODE), enviando a $VPS_URL"

# ---------- 7. POST al MCP via curl (initialize + tools/call) ----------
# Streamable HTTP MCP: initialize primero, después tools/call.
curl -fsS -X POST "${VPS_URL}/mcp" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"domain-code-graph","version":"0.2"}}}' \
  >/dev/null 2>&1 || true

# tools/call: leer el grafo desde GRAPH_FILE y construir el request MCP
python3 -c "
import json,sys
with open('$GRAPH_FILE') as f:
    payload = json.load(f)
with open('$REQ_FILE','w') as f:
    json.dump({
        'jsonrpc':'2.0','id':2,
        'method':'tools/call',
        'params':{
            'name':'domain_code_upload',
            'arguments': payload,
        }
    }, f)
"
RESP=$(curl -fsS -X POST "${VPS_URL}/mcp" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d "@${REQ_FILE}")

# respuesta puede ser JSON o SSE; el server usa 'result.content[0].text' con JSON adentro.
# exit != 0 si el server reporto error — el caller (hook) y el estado incremental
# dependen de distinguir exito real.
echo "$RESP" | python3 -c "
import json, sys
try:
    d = json.load(sys.stdin)
    if 'error' in d:
        print('ERROR:', d['error'].get('message', d['error']))
        sys.exit(1)
    res = d.get('result', {})
    # el server mete el JSON en result.content[0].text
    text = ''
    for c in res.get('content', []):
        text += c.get('text', '')
    if not text and 'stats' in res:
        text = json.dumps(res)
    if 'failed' in text.lower() or res.get('isError'):
        print('[upload] ERROR del server:', text[:300])
        sys.exit(1)
    print('[upload] OK', text[:500])
except SystemExit:
    raise
except Exception as e:
    print('ERROR parseando respuesta:', e)
    print('raw:', sys.stdin.read()[:300])
    sys.exit(1)
"
UPLOAD_STATUS=$?

# ---------- 8. persistir el head subido (habilita el proximo run incremental) ----------
if [ "$UPLOAD_STATUS" -eq 0 ] && [ -n "$GIT_HEAD" ]; then
  mkdir -p "$STATE_DIR" 2>/dev/null
  printf '%s' "$GIT_HEAD" > "$STATE_FILE" 2>/dev/null && \
    echo "[state] head ${GIT_HEAD:0:12} guardado — proximo run sera incremental"
fi
exit "$UPLOAD_STATUS"
