#!/usr/bin/env bash
# scripts/domain-code-graph.sh
#
# Construye el code graph de un repo LOCAL y lo sube al server domain-mcp.
# Reemplaza a domain_code_build server-side: domain-mcp corre en el VPS y no
# tiene acceso al filesystem del cliente, asi que el parseo se hace aca con
# ast-grep (sg) y se sube via la tool MCP domain_code_upload.
#
# Multi-lenguaje via ast-grep: TypeScript/JavaScript/TSX/JSX/Go/Python/Rust/
# Java. Si sg no esta instalado, lo instala segun distro (pacman/apt/brew/cargo).
#
# Uso:
#   domain-code-graph.sh [REPO_PATH] [PROJECT_SLUG]
#   - REPO_PATH: path absoluto al repo (default: cwd)
#   - PROJECT_SLUG: slug del project en domain (default: basename del repo)
#
# Output: JSON con {nodes, edges, files} + POST a domain-mcp via MCP HTTP.
#
# Requiere: ast-grep (sg) instalado (script lo instala si falta), curl, jq opcional.

set -uo pipefail

# NO usamos `set -e` porque ast-grep puede retornar !=0 para archivos que no
# matchean patterns, y eso mataría el script entero. En su lugar, manejamos
# errores explicitamente con `|| true` donde corresponde.

REPO_PATH="${1:-$(pwd)}"
PROJECT_SLUG="${2:-$(basename "$REPO_PATH")}"

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

# ---------- 4. descubrir archivos por lenguaje (skip vendor/node_modules/dist/build/.git) ----------
mapfile -t FILES < <(find "$REPO_PATH" -type f \
  \( -name "*.go" -o -name "*.ts" -o -name "*.tsx" -o -name "*.js" -o -name "*.jsx" -o -name "*.py" -o -name "*.rs" -o -name "*.java" \) \
  -not -path "*/node_modules/*" -not -path "*/dist/*" -not -path "*/build/*" -not -path "*/.git/*" -not -path "*/vendor/*" \
  2>/dev/null)

if [ ${#FILES[@]} -eq 0 ]; then
  echo "[scan] no encontre archivos a parsear en $REPO_PATH"
  exit 0
fi
echo "[scan] ${#FILES[@]} archivos a parsear"

# ---------- 5. extraer nodos y edges con ast-grep ----------
# Solo corre patterns para los lenguajes realmente presentes (ahorra ~70%
# de invocaciones). Detecta extensión desde FILES.
NODES_FILE=$(mktemp)
EDGES_FILE=$(mktemp)
GRAPH_FILE=$(mktemp)
REQ_FILE=$(mktemp)
trap 'rm -f "$NODES_FILE" "$EDGES_FILE" "$GRAPH_FILE" "$REQ_FILE"' EXIT

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
  printf '%s\0' "${FILES[@]}" | xargs -0 -I{} ast-grep run \
    --pattern "$pattern" --lang "$lang" --json=compact {} 2>/dev/null | \
    python3 -c "
import sys, json, os
pat_kind = '$kind'
data = sys.stdin.read()
decoder = json.JSONDecoder()
idx = 0
entries = []
while idx < len(data):
    while idx < len(data) and data[idx] in ' \n\r\t':
        idx += 1
    if idx >= len(data): break
    try:
        obj, end = decoder.raw_decode(data, idx)
        if isinstance(obj, list):
            entries.extend(obj)
        else:
            entries.append(obj)
        idx = end
    except json.JSONDecodeError:
        break
for m in entries:
        file_path = m.get('file','')
        if file_path.startswith('$REPO_PATH/'):
            rel = file_path[len('$REPO_PATH/'):]
        else:
            rel = file_path
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

# ast-grep es el binario real (sg es /usr/bin/sg = group, no ast-grep).
# Definimos ASTGREP_CMD para que las llamadas sg scan usen el binario correcto.
ASTGREP_CMD="ast-grep"

# Edges: buscar llamadas (heurística: nombre seguido de '(').
# El target_qn lo envía como nombre simple; el server resuelve por nombre
# (fallback en resolveTarget: primero qualified_name exacto, luego name search).
printf '%s\0' "${FILES[@]}" > "$EDGES_FILE.files"
python3 -c "
import re, sys, os
repo_prefix = '$REPO_PATH'
if not repo_prefix.endswith('/'):
    repo_prefix += '/'
files_path = '$EDGES_FILE.files'
with open(files_path, 'rb') as fh:
    raw = fh.read()
paths = raw.rstrip(b'\x00').split(b'\x00')
for fp_bytes in paths:
    fp = fp_bytes.decode('utf-8', errors='replace')
    rel = fp[len(repo_prefix):] if fp.startswith(repo_prefix) else fp
    try:
        lines = open(fp, 'r', encoding='utf-8', errors='ignore').readlines()
    except:
        continue
    for lineno, line in enumerate(lines, 1):
        for m in re.finditer(r'\\b([a-zA-Z_][a-zA-Z0-9_]*)\\s*\\(', line):
            name = m.group(1)
            if name in ('if', 'for', 'while', 'switch', 'return', 'function', 'func', 'def', 'class'):
                continue
            print(f'{rel}\t{lineno}\t{name}')
" > "$EDGES_FILE" 2>/dev/null
rm -f "$EDGES_FILE.files"

NODES_COUNT=$(wc -l < "$NODES_FILE" || echo 0)
EDGES_COUNT=$(wc -l < "$EDGES_FILE" || echo 0)
echo "[parse] $NODES_COUNT nodos, $EDGES_COUNT edges detectados"

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
edges=[]
# Indexar nodos por file → [(line_start, line_end, qn)], ordenado por line_start
nodes_by_file = {}
with open('$NODES_FILE') as nf:
    for line in nf:
        parts=line.rstrip('\n').split('\t')
        if len(parts)<6: continue
        rel,name,qn,ls_s,le_s = parts[0],parts[2],parts[3],parts[4],parts[5]
        try: ls_int=int(ls_s)
        except: ls_int=0
        try: le_int=int(le_s)
        except: le_int=0
        nodes_by_file.setdefault(rel, []).append((ls_int, le_int, qn or ''))
# Ordenar por line_start ascendente
for f in nodes_by_file:
    nodes_by_file[f].sort()
seen_edges=set()
with open('$EDGES_FILE') as f:
    for line in f:
        parts=line.rstrip('\n').split('\t')
        if len(parts)<3: continue
        rel,ls_str,target=parts
        try: edge_line=int(ls_str)
        except: continue
        # source_qn = nodo que contiene esta línea (func con mayor line_start <= edge_line)
        src_qn = ''
        funcs = nodes_by_file.get(rel, [])
        # búsqueda binaria simplificada: recorrer hacia atrás desde el techo
        # (funcs son pocas por archivo, O(n) es aceptable)
        for f_ls, f_le, f_qn in reversed(funcs):
            if f_ls <= edge_line and (f_le == 0 or edge_line <= f_le):
                src_qn = f_qn
                break
        if not src_qn:
            continue
        key = (src_qn, target, 'calls')
        if key in seen_edges: continue
        seen_edges.add(key)
        edges.append({'source_qn':src_qn,'target_qn':target,'edge_type':'calls'})
# El tool espera project_slug + graph_json:{files_scanned,git_head,nodes,edges}
with open('$GRAPH_FILE','w') as f:
    json.dump({
        'project_slug':'$PROJECT_SLUG',
        'graph_json': {
            'git_head':'$GIT_HEAD',
            'files_scanned':$FILE_COUNT,
            'nodes':nodes,
            'edges':edges,
        }
    }, f)
"

echo "[upload] $NODES_COUNT nodos + $EDGES_COUNT edges de $FILE_COUNT archivos, enviando a $VPS_URL"

# ---------- 7. POST al MCP via curl (initialize + tools/call) ----------
# Streamable HTTP MCP: initialize primero, después tools/call.
curl -fsS -X POST "${VPS_URL}/mcp" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"domain-code-graph","version":"0.1"}}}' \
  >/dev/null 2>&1 || true

# tools/call con el grafo como argumento
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

# respuesta puede ser JSON o SSE; el server usa 'result.content[0].text' con JSON adentro
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
    print('[upload] OK', text[:500])
except Exception as e:
    print('ERROR parseando respuesta:', e)
    print('raw:', sys.stdin.read()[:300])
"