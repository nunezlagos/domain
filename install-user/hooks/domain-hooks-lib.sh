#!/usr/bin/env bash
# hooks/domain-hooks-lib.sh — helpers compartidos por los hooks de domain
# (domain-user-prompt.sh, domain-stop.sh). Mismo orden de resolución de
# credenciales que domain-session-start.sh: env > install.env > .env de
# clientes. Todo best-effort: los hooks NUNCA deben bloquear la sesión.

# domain_resolve_env setea $vps_url y $api_key. Retorna 1 si faltan.
domain_resolve_env() {
  vps_url="${DOMAIN_VPS_URL:-}"
  api_key="${DOMAIN_API_KEY:-}"
  if [ -z "$vps_url" ] || [ -z "$api_key" ]; then
    for envf in "$HOME/.config/domain/install.env" "$HOME/.claude/.env" "$HOME/.config/opencode/.env"; do
      [ -r "$envf" ] || continue
      while IFS='=' read -r k v; do
        kk="${k#DOMAIN_}"
        v="${v%\"}"; v="${v#\"}"
        v="${v%\'}"; v="${v#\'}"
        case "$kk" in
          VPS_URL)             [ -z "$vps_url" ] && vps_url="$v" ;;
          MCP_API_KEY|API_KEY) [ -z "$api_key" ] && api_key="$v" ;;
        esac
      done < "$envf"
    done
  fi
  [ -n "$vps_url" ] && [ -n "$api_key" ]
}

# domain_mcp_init manda el initialize (el server sessionless responde igual,
# pero algunos setups lo exigen antes del tools/call).
domain_mcp_init() {
  # DOMAINSERV-77: el Bearer va por -K <(printf ...) y no por -H, para que no
  # aparezca en argv (ps/proc). printf es builtin de bash → sin proceso propio.
  curl -fsS -m 4 -X POST "${vps_url}/mcp" \
    -K <(printf 'header = "Authorization: Bearer %s"\n' "$api_key") \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"domain-lifecycle-hook","version":"0.1"}}}'
}

# domain_call_tool <tool_name> <args_json> — imprime la respuesta JSON-RPC.
domain_call_tool() {
  # DOMAINSERV-77: Bearer por -K (fuera de argv), ver domain_mcp_init.
  curl -fsS -m 6 -X POST "${vps_url}/mcp" \
    -K <(printf 'header = "Authorization: Bearer %s"\n' "$api_key") \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"$1\",\"arguments\":$2}}"
}

# domain_tests_code_hash — DOMAINSERV-219. Hash del CONTENIDO del árbol de
# código, para que el commit-gate sepa si la corrida de tests sigue cubriendo el
# estado actual. Reemplaza a sha256(git diff --no-color HEAD), que fallaba por
# tres lados: (a) cambiaba al tocar un archivo inerte como el CHANGELOG,
# (b) cambiaba con CADA commit porque HEAD se mueve, así que el segundo commit
# del mismo lote nunca reusaba la corrida, y (c) no veía los untracked — un .go
# nuevo sin `git add` NO invalidaba el marker (falso verde).
#
# Se hashea contenido del working tree (no del index), así que `git add` tampoco
# lo mueve. Único inerte: .md fuera de templates/ y testdata/. Los .md de
# templates/ son código (install-user/embed.go los mete en go:embed y
# agent_voseo_test.go los aserta) y los de testdata/ son fixtures. Cualquier otra
# extensión invalida el marker por default: fail-closed.
domain_tests_code_hash() {
  local raiz lista
  raiz=$(git rev-parse --show-toplevel 2>/dev/null)
  [ -n "$raiz" ] || return 1
  # git no admite re-inclusión después de un :(exclude) en la misma llamada, de
  # ahí las dos pasadas
  lista=$(cd "$raiz" 2>/dev/null && {
    git ls-files -co --exclude-standard -- ':(exclude)*.md'
    git ls-files -co --exclude-standard -- '*templates/*.md' '*testdata/*.md'
  # DOMAINSERV-247: LC_ALL=C NO es cosmético. `sort` de GNU con un locale como es_CL.UTF-8 ignora
  # la puntuación al comparar, así que el ORDEN de la lista —y por lo tanto el hash— dependía del
  # LANG de quien corriera el hook. MEDIDO: el mismo repo da 3f32e5d8... con LC_ALL=C y
  # 08391a55... con es_CL.UTF-8. Si el post-test escribía el marker con un locale y el pre-edit
  # comparaba con otro, el gate denegaba sin ninguna razón visible.
  } | while IFS= read -r f; do [ -f "$f" ] && printf '%s\n' "$f"; done | LC_ALL=C sort)
  [ -n "$lista" ] || return 1
  # los nombres van junto a los hashes: sin eso un rename o un borrado pasaría inadvertido
  {
    printf '%s\n' "$lista"
    printf '%s\n' "$lista" | (cd "$raiz" && git hash-object --stdin-paths 2>/dev/null)
  } | sha256sum 2>/dev/null | cut -d' ' -f1
}

# domain_log_injection <hook> <session_id> <resumen> — REQ-55 issue-55.5.
# additionalContext de los hooks es invisible en la UI de Claude Code (sin log
# nativo). Dejamos rastro auditable en ~/.local/state/domain/injections.log:
# timestamp ISO, hook, session, resumen del contexto inyectado. Best-effort:
# nunca falla ni bloquea la sesión (todo redirigido, siempre "return 0").
domain_log_injection() {
  local dir="$HOME/.local/state/domain"
  mkdir -p "$dir" 2>/dev/null || return 0
  local ts summary
  ts=$(date -Iseconds 2>/dev/null || echo "?")
  # resumen en una línea, recortado a 200 chars (el log es índice, no copia)
  summary=$(printf '%s' "$3" | tr '\n' ' ' | cut -c1-200)
  printf '%s\t%s\t%s\t%s\n' "$ts" "$1" "${2:-?}" "$summary" >> "$dir/injections.log" 2>/dev/null
  return 0
}

# domain_log_hook_error <hook> <session_id> <op> <detail> — REQ-56 issue-56.2.
# Los hooks lifecycle silencian sus propios fallos (curl a domain_* falla → se
# descarta con >/dev/null 2>&1). Esto deja rastro auditable de esos fallos en
# ~/.local/state/domain/hook-errors.log: timestamp, hook, session, operación que
# falló y un detalle corto. Best-effort: nunca falla ni bloquea la sesión.
domain_log_hook_error() {
  local dir="$HOME/.local/state/domain"
  mkdir -p "$dir" 2>/dev/null || return 0
  local ts detail
  ts=$(date -Iseconds 2>/dev/null || echo "?")
  detail=$(printf '%s' "$4" | tr '\n' ' ' | cut -c1-300)
  printf '%s\t%s\t%s\t%s\t%s\n' "$ts" "$1" "${2:-?}" "${3:-?}" "$detail" >> "$dir/hook-errors.log" 2>/dev/null
  return 0
}

# domain_test_cmd_sugerido <dir> — DOMAINSERV-265. El comando de tests que
# corresponde al stack de <dir>, o vacío si no se reconoce ninguno.
#
# Vive acá y no en el pre-edit porque el post-test ya acepta 8 runners y el
# pre-edit hablaba de uno solo: esa divergencia entre las dos puntas del gate ES
# el bug. Un repo Node recibía "corré go test", concluía que el gate no soportaba
# su stack, y se iba al bypass con la suite perfectamente ejecutable.
#
# Vacío NO significa "usá go test": significa que el mensaje debe admitir que no
# sabe, para que el bypass sea una decisión informada y no un reflejo.
domain_test_cmd_sugerido() {
  local dir="${1:-}"
  [ -n "$dir" ] && [ -d "$dir" ] || return 0
  # go primero: el gate ya trata a Go distinto del resto al medir el alcance de
  # una corrida (DOMAINSERV-237), así que en un monorepo mixto manda el mismo eje
  [ -f "$dir/go.mod" ]         && { printf 'go test -count=1 ./...'; return 0; }
  [ -f "$dir/package.json" ]   && { printf 'npm test'; return 0; }
  [ -f "$dir/pyproject.toml" ] && { printf 'pytest'; return 0; }
  [ -f "$dir/Cargo.toml" ]     && { printf 'cargo test'; return 0; }
  [ -f "$dir/composer.json" ]  && { printf 'phpunit'; return 0; }
  [ -f "$dir/Gemfile" ]        && { printf 'rspec'; return 0; }
  # DOMAINSERV-254: un repo sin manifest de stack puede tener igual una suite ejecutable
  # —este mismo repo la tiene en scripts/tests/— y el post-test ya la reconoce. Callar acá
  # hacía que el deny la declarara "de otro tipo" y mandara al bypass una suite que el
  # gate podía aceptar: las dos puntas divergían, que es exactamente el bug de arriba.
  local f
  for f in "$dir"/scripts/tests/test_*.sh "$dir"/*_test.sh; do
    [ -f "$f" ] || continue
    printf 'bash %s' "${f#"$dir"/}"
    return 0
  done
  return 0
}
