#!/usr/bin/env bash
# REQ-65: Benchmark MCP HTTP vs SQL directo.
#
# Compara la latencia wall-clock de queries equivalentes:
#   - via MCP tool      (HTTP + JSON-RPC + auth + RLS tx wrapper)
#   - via SQL directo   (psql sobre el mismo Postgres)
#
# Asume:
#   - Domain backend corriendo en $DOMAIN_URL (default http://localhost:8080)
#   - API key Bearer en $DOMAIN_API_KEY
#   - psql conectable con $DOMAIN_DATABASE_URL (o DATABASE_URL)
#   - Demo data ya seedeada (`domain seed-demo <org-id>`)
#
# Uso:
#   DOMAIN_API_KEY=domk_test_xxx \
#   DOMAIN_DATABASE_URL='postgres://domain:domain@localhost:5432/domain' \
#   ORG_ID=721e9026-f703-4eb1-ab43-c2bf52978f33 \
#   PROJECT_SLUG=acme-api \
#   ./scripts/bench_mcp_vs_sql.sh [iterations]
#
# iterations: cuántas veces ejecutar cada query (default 20).

set -euo pipefail

DOMAIN_URL=${DOMAIN_URL:-http://localhost:8080}
API_KEY=${DOMAIN_API_KEY:?DOMAIN_API_KEY no seteado}
DSN=${DOMAIN_DATABASE_URL:-${DATABASE_URL:-}}
if [[ -z "$DSN" ]]; then
  echo "ERROR: DOMAIN_DATABASE_URL no seteado" >&2
  exit 1
fi
ORG_ID=${ORG_ID:?ORG_ID no seteado}
PROJECT_SLUG=${PROJECT_SLUG:-acme-api}
N=${1:-20}

command -v jq    >/dev/null || { echo "jq requerido"; exit 1; }
command -v psql  >/dev/null || { echo "psql requerido"; exit 1; }
command -v curl  >/dev/null || { echo "curl requerido"; exit 1; }
command -v bc    >/dev/null || { echo "bc requerido"; exit 1; }

# Resolver project_id de SQL para la query directa.
PROJECT_ID=$(psql "$DSN" -tA -c \
  "SELECT id FROM projects WHERE organization_id='$ORG_ID' AND slug='$PROJECT_SLUG' AND deleted_at IS NULL")
if [[ -z "$PROJECT_ID" ]]; then
  echo "ERROR: project '$PROJECT_SLUG' no existe en org '$ORG_ID'" >&2
  exit 1
fi

echo "==> Benchmark MCP vs SQL"
echo "    DOMAIN_URL    : $DOMAIN_URL"
echo "    ORG_ID        : $ORG_ID"
echo "    PROJECT_SLUG  : $PROJECT_SLUG ($PROJECT_ID)"
echo "    iterations    : $N"
echo

# ms-resolution timer usando date +%s%N.
now_ms() { date +%s%3N; }

# stats: prints "min/avg/max ms" de una lista de números separados por espacio
stats() {
  local nums=($1)
  local min=${nums[0]} max=${nums[0]} sum=0
  for v in "${nums[@]}"; do
    (( v < min )) && min=$v
    (( v > max )) && max=$v
    sum=$((sum + v))
  done
  local avg=$(echo "scale=1; $sum/${#nums[@]}" | bc)
  echo "$min / $avg / $max ms"
}

mcp_call() {
  local tool=$1 args=$2
  local payload
  payload=$(jq -nc --arg name "$tool" --argjson args "$args" '{
    jsonrpc:"2.0", id:1, method:"tools/call",
    params:{name:$name, arguments:$args}
  }')
  curl -sS -X POST "$DOMAIN_URL/mcp" \
    -H "Authorization: Bearer $API_KEY" \
    -H 'Content-Type: application/json' \
    -d "$payload" > /dev/null
}

sql_call() {
  psql "$DSN" -tAq -c "$1" > /dev/null
}

bench_pair() {
  local label=$1
  local mcp_tool=$2 mcp_args=$3
  local sql=$4

  echo "--- $label ---"
  local mcp_times="" sql_times=""

  # warm-up
  mcp_call "$mcp_tool" "$mcp_args" || true
  sql_call "$sql" || true

  for ((i=0; i<N; i++)); do
    local t0=$(now_ms)
    mcp_call "$mcp_tool" "$mcp_args"
    local t1=$(now_ms)
    mcp_times+=" $((t1 - t0))"

    t0=$(now_ms)
    sql_call "$sql"
    t1=$(now_ms)
    sql_times+=" $((t1 - t0))"
  done
  printf "  MCP : %s\n" "$(stats "$mcp_times")"
  printf "  SQL : %s\n" "$(stats "$sql_times")"
  echo
}

# --- queries equivalentes ---

bench_pair "list tickets (all status, project=$PROJECT_SLUG)" \
  "domain_ticket_list" \
  "$(jq -nc --arg s "$PROJECT_SLUG" '{project_slug:$s, limit:50}')" \
  "SET app.current_org_id = '$ORG_ID';
   SELECT id,key,title,status,priority FROM project_tickets
   WHERE organization_id='$ORG_ID' AND project_id='$PROJECT_ID'
     AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 50;"

bench_pair "list policies (project=$PROJECT_SLUG)" \
  "domain_policy_list" \
  "$(jq -nc --arg s "$PROJECT_SLUG" '{project_slug:$s}')" \
  "SET app.current_org_id = '$ORG_ID';
   SELECT slug,name,kind FROM project_policies
   WHERE organization_id='$ORG_ID' AND project_id='$PROJECT_ID'
     AND is_active=true AND deleted_at IS NULL;"

bench_pair "health" \
  "domain_health" "{}" \
  "SET app.current_org_id = '$ORG_ID';
   SELECT COUNT(*) FROM project_tickets WHERE organization_id='$ORG_ID';"

bench_pair "captured_prompts recent (project=acme-web)" \
  "domain_prompt_captured_list" \
  '{"limit":20}' \
  "SET app.current_org_id = '$ORG_ID';
   SELECT id,content,captured_at FROM captured_prompts
   WHERE organization_id='$ORG_ID' ORDER BY captured_at DESC LIMIT 20;"

echo "==> Done."
echo
echo "Lectura del resultado:"
echo "  - MCP suma: TCP→TLS (si aplica)→auth→tx wrapper→handler→serialización JSON."
echo "  - SQL suma: psql startup + 1 round-trip."
echo "  - Overhead típico de MCP: 5-20ms por call (auth + wrapper + JSON)."
echo "  - SQL local debería ser 1-5ms; psql startup pesa ~30ms por invocación,"
echo "    por eso a veces SQL 'pierde' acá. Para una comparación pura, usá"
echo "    un cliente persistente (PgBench, ab+ngx) en lugar de psql cold."
