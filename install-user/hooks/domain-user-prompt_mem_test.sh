#!/usr/bin/env bash
# Test del bloque de parseo de memorias de domain-user-prompt.sh (DOMAINSERV-161).
#
# Antes de esto NO había ningún test que ejercitara el parseo real: el único test de
# hooks (domain-session-start_host_test.sh:27) mockea HOOK_MEM_OUT="" y nunca lo toca.
# Ese es el motivo de que un cambio de shape río arriba pudiera romper la inyección de
# memorias en silencio — el bloque es best-effort y no loguea nada.
#
# Extrae el bloque python del hook y lo alimenta con respuestas MCP reales.
# Correr: bash install-user/hooks/domain-user-prompt_mem_test.sh

set -uo pipefail
cd "$(dirname "$0")"

HOOK="domain-user-prompt.sh"
fallos=0

# el bloque vive entre la línea del python3 -c y el cierre de la comilla simple
extraer_parser() {
  sed -n "/mem_msg=\$(cat \"\$_memf\" 2>\/dev\/null | python3 -c '/,/^' 2>\/dev\/null)\$/p" "$HOOK" \
    | sed '1d;$d'
}

PARSER="$(extraer_parser)"
if [ -z "$PARSER" ]; then
  echo "FAIL setup: no se pudo extraer el bloque de parseo de $HOOK"
  echo "un error de setup es un fallo, nunca un skip"
  exit 1
fi

verificar() {
  local nombre="$1" entrada="$2" esperado="$3" modo="${4:-contiene}"
  local salida
  salida="$(printf '%s' "$entrada" | python3 -c "$PARSER" 2>/dev/null)"

  case "$modo" in
    contiene)
      if printf '%s' "$salida" | grep -qF -- "$esperado"; then
        echo "PASS $nombre"
      else
        echo "FAIL $nombre"
        echo "  esperaba que contuviera: $esperado"
        echo "  salida: ${salida:-<vacía>}"
        fallos=$((fallos + 1))
      fi
      ;;
    no_contiene)
      if printf '%s' "$salida" | grep -qF -- "$esperado"; then
        echo "FAIL $nombre"
        echo "  no debía contener: $esperado"
        echo "  salida: $salida"
        fallos=$((fallos + 1))
      else
        echo "PASS $nombre"
      fi
      ;;
    vacio)
      if [ -z "$salida" ]; then
        echo "PASS $nombre"
      else
        echo "FAIL $nombre"
        echo "  esperaba salida vacía, obtuvo: $salida"
        fallos=$((fallos + 1))
      fi
      ;;
  esac
}

envelope() { # $1 = body JSON del tool result
  printf '{"result":{"content":[{"type":"text","text":%s}]}}' "$(printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g' | awk '{printf "\"%s\"", $0}')"
}

# ── el shape NUEVO: content acotado a 200 + content_len con el largo real ────────────
nuevo='{"results":[{"id":"11111111-1111-1111-1111-111111111111","content":"una decision importante sobre el shape","content_len":4820,"observation_type":"decision"}],"count":1}'
verificar "shape nuevo con content_len sigue inyectando memorias" "$(envelope "$nuevo")" "memorias relevantes a este prompt"
verificar "shape nuevo preserva el texto del snippet" "$(envelope "$nuevo")" "una decision importante sobre el shape"

# ── el marcador del helper truncate no debe filtrarse al prompt inyectado ────────────
con_marcador='{"results":[{"id":"22222222-2222-2222-2222-222222222222","content":"texto que venia cortado ...[truncated]","content_len":9000}],"count":1}'
verificar "el marcador de truncado no llega al prompt" "$(envelope "$con_marcador")" "[truncated]" no_contiene
verificar "pero el texto previo al marcador sí" "$(envelope "$con_marcador")" "texto que venia cortado"

# ── compatibilidad: el shape VIEJO tiene que seguir funcionando ──────────────────────
viejo='{"results":[{"id":"33333333-3333-3333-3333-333333333333","content":"memoria con el shape viejo entero"}],"count":1}'
verificar "shape viejo sin content_len no se rompe" "$(envelope "$viejo")" "memoria con el shape viejo entero"

# ── degradaciones: ninguna debe emitir un bloque a medias ────────────────────────────
verificar "results vacío no inyecta nada" "$(envelope '{"results":[],"count":0}')" "" vacio
verificar "content vacío no inyecta nada" "$(envelope '{"results":[{"id":"x","content":"","content_len":0}]}')" "" vacio
verificar "JSON inválido no inyecta nada" 'no soy json' "" vacio
verificar "envelope sin content no inyecta nada" '{"result":{}}' "" vacio

# ── DOMAINSERV-145: el id tiene que llegar al prompt inyectado ───────────────────────
# El hook inyectaba el contenido y descartaba el id, así que el agente no tenía con qué
# llamar a domain_mem_used: la tool pedía un observation_id que nunca había visto. El id
# va COMPLETO y no abreviado, porque la tool exige UUIDs y un prefijo no sirve.
verificar "el id de la observación se inyecta completo" "$(envelope "$nuevo")" "11111111-1111-1111-1111-111111111111"
verificar "el id acompaña a su texto" "$(envelope "$nuevo")" "11111111-1111-1111-1111-111111111111 una decision importante sobre el shape"

dos='{"results":[{"id":"aaaaaaaa-1111-1111-1111-111111111111","content":"primera memoria"},{"id":"bbbbbbbb-2222-2222-2222-222222222222","content":"segunda memoria"}],"count":2}'
verificar "con varias memorias se inyectan todos los ids (1/2)" "$(envelope "$dos")" "aaaaaaaa-1111-1111-1111-111111111111"
verificar "con varias memorias se inyectan todos los ids (2/2)" "$(envelope "$dos")" "bbbbbbbb-2222-2222-2222-222222222222"

# Una observación sin id sigue aportando contexto: se inyecta el texto igual. Perder la
# memoria entera por no poder reportarla sería peor que no poder reportarla.
sin_id='{"results":[{"content":"memoria sin id que igual sirve"}],"count":1}'
verificar "una memoria sin id igual se inyecta" "$(envelope "$sin_id")" "memoria sin id que igual sirve"

# domain_mem_used declara DOS parámetros required: prompt_id y candidate_ids. Inyectar
# solo los ids de observaciones deja la tool igual de inalcanzable, con el otro required
# faltando. El hook YA tiene el prompt_id — lo escribe en turn-<session_id>.id para el
# hook Stop — así que acá solo hay que pasárselo también al agente.
DOMAIN_TURN_PID="99999999-9999-9999-9999-999999999999" \
  verificar "el prompt_id se inyecta cuando el hook lo tiene" "$(envelope "$nuevo")" "99999999-9999-9999-9999-999999999999"

verificar "sin prompt_id no se inventa nada" "$(envelope "$nuevo")" "prompt_id=" no_contiene

echo
if [ "$fallos" -gt 0 ]; then
  echo "$fallos test(s) FALLARON"
  exit 1
fi
echo "todos los tests del parseo de memorias pasaron"
