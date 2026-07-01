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

set -euo pipefail

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
# Formato del output: cada linea es un JSON con kind, name, qualified_name, line_start, line_end, file_path.
NODES_FILE=$(mktemp)
EDGES_FILE=$(mktemp)
trap 'rm -f "$NODES_FILE" "$EDGES_FILE"' EXIT

extract_for_lang() {
  local lang="$1" pattern="$2" kind="$3"
  # ast-grep scan --pattern '...' --lang X --json=compact imprime matches como JSON lines
  ast-grep scan --pattern "$pattern" --lang "$lang" --json=compact 2>/dev/null | \
    python3 -c "
import sys, json, os
pat_kind = '$kind'
for line in sys.stdin:
    line = line.strip()
    if not line: continue
    try:
        m = json.loads(line)
    except: continue
    file_path = m.get('file','')
    if file_path.startswith('$REPO_PATH/'):
        rel = file_path[len('$REPO_PATH/'):]
    else:
        rel = file_path
    text = m.get('text','').split('\n')[0]
    line_start = m.get('range',{}).get('start',{}).get('line', 0) + 1
    line_end = m.get('range',{}).get('end',{}).get('line', line_start)
    # nombre: limpiar el match (sacar parens, braces, return type, etc)
    name = text.split('(')[0].split('{')[0].strip()
    # para class: 'class Foo extends Bar' -> nombre 'Foo'
    for kw in ('class ', 'function ', 'func ', 'def ', 'fn ', 'export function ', 'export const ', 'export default function ', 'async function ', 'public function ', 'private function '):
        if name.startswith(kw):
            name = name[len(kw):].strip()
            break
    # qualified_name: path:line:Nombre
    qn = f'{rel}:{line_start}:{name}' if name else f'{rel}:{line_start}'
    print(f'{rel}\t{pat_kind}\t{name}\t{qn}\t{line_start}\t{line_end}\t{text}')
" 2>/dev/null
}

# Patterns por lenguaje (ast-grep usa tree-sitter grammars built-in)
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

# Edges: buscar llamadas (heurística: nombre seguido de '(')
> "$EDGES_FILE"
for f in "${FILES[@]}"; do
  rel="${f#$REPO_PATH/}"
  # extraer identificadores seguidos de '(' (posibles calls)
  python3 -c "
import re, sys
with open('$f', 'r', encoding='utf-8', errors='ignore') as fh:
    lines = fh.readlines()
for i, line in enumerate(lines, 1):
    # matches: word boundary + name + (
    for m in re.finditer(r'\\b([a-zA-Z_][a-zA-Z0-9_]*)\\s*\\(', line):
        name = m.group(1)
        if name in ('if', 'for', 'while', 'switch', 'return', 'function', 'func', 'def', 'class'):
            continue
        print(f'{rel}\t{i}\t{name}')
" >> "$EDGES_FILE" 2>/dev/null
done

NODES_COUNT=$(wc -l < "$NODES_FILE" || echo 0)
EDGES_COUNT=$(wc -l < "$EDGES_FILE" || echo 0)
echo "[parse] $NODES_COUNT nodos, $EDGES_COUNT edges detectados"

# ---------- 6. construir el JSON del grafo (vía python, robusto) ----------
FILE_COUNT=${#FILES[@]}
PAYLOAD=$(python3 -c "
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
with open('$EDGES_FILE') as f:
    for line in f:
        parts=line.rstrip('\n').split('\t')
        if len(parts)<3: continue
        rel,ls,target=parts
        edges.append({'source_qn':f'{rel}:{ls}','target_qn':target,'edge_type':'calls'})
print(json.dumps({
    'project_slug':'$PROJECT_SLUG',
    'git_head':'$GIT_HEAD',
    'files_scanned':$FILE_COUNT,
    'nodes':nodes,
    'edges':edges,
}))
")

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
RESP=$(curl -fsS -X POST "${VPS_URL}/mcp" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d "$(python3 -c "
import json,sys
payload = json.loads('''$PAYLOAD''')
print(json.dumps({
  'jsonrpc':'2.0','id':2,
  'method':'tools/call',
  'params':{
    'name':'domain_code_upload',
    'arguments': payload,
  }
}))
")")

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