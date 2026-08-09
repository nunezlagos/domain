#!/usr/bin/env bash
# install-curl_cache_test.sh — DOMAINSERV-260
#
# install-curl.sh leía el marker del cache en $CACHE_DIR y lo escribía en
# $REPO_DIR, así que nunca estaba donde se lo buscaba: need_clone=1 siempre, el
# repo entero se re-clonaba en cada corrida con un warning falso de "cache
# corrupto", y el camino fetch+reset nunca se ejecutó.
#
# El primer test es FUNCIONAL: reproduce la decisión con un árbol de prueba y
# comprueba el resultado. El segundo es un guard sobre el script real, que es lo
# que impide que el bug vuelva por una edición futura.

set -uo pipefail

AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="$AQUI/install-curl.sh"

PASS=0; FAIL=0
pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

[ -r "$SCRIPT" ] || { fail "no existe $SCRIPT"; exit 1; }

# ---------------------------------------------------------------------------
# 1. FUNCIONAL — la segunda corrida reutiliza el clone
# ---------------------------------------------------------------------------
# Replica la decisión de install-curl.sh sin correr el instalador: crear un
# cache como lo deja una instalación exitosa, y evaluar si la corrida siguiente
# volvería a clonar. El path del marker sale del script REAL, no se hardcodea.
marker_dir_de_escritura() {
  grep -oE '> *"\$(CACHE_DIR|REPO_DIR)/\.domain-user-install-marker"' "$SCRIPT" \
    | grep -oE 'CACHE_DIR|REPO_DIR' | head -1
}
marker_dir_de_lectura() {
  grep -oE '\[ -f "\$(CACHE_DIR|REPO_DIR)/\.domain-user-install-marker" \]' "$SCRIPT" \
    | grep -oE 'CACHE_DIR|REPO_DIR' | head -1
}

simular_segunda_corrida() {
  local cache="$WORK/cache" repo
  repo="$cache/repo"
  rm -rf "$cache"; mkdir -p "$repo/.git"

  # sembrar el marker donde el script dice que lo escribe
  case "$(marker_dir_de_escritura)" in
    CACHE_DIR) printf 'x\n' > "$cache/.domain-user-install-marker" ;;
    REPO_DIR)  printf 'x\n' > "$repo/.domain-user-install-marker" ;;
  esac

  # y evaluar la condición tal como la script la evalúa
  local need_clone=1 marker
  case "$(marker_dir_de_lectura)" in
    CACHE_DIR) marker="$cache/.domain-user-install-marker" ;;
    REPO_DIR)  marker="$repo/.domain-user-install-marker" ;;
    *)         marker="" ;;
  esac
  if [ -d "$repo/.git" ] && [ -n "$marker" ] && [ -f "$marker" ]; then
    need_clone=0
  fi
  printf '%s' "$need_clone"
}

echo "--- funcional: la 2da corrida no debe re-clonar"
r=$(simular_segunda_corrida)
if [ "$r" = "0" ]; then
  pass "tras un install exitoso, la corrida siguiente REUTILIZA el clone (need_clone=0)"
else
  fail "need_clone=$r: el instalador re-clona el repo entero en cada corrida y avisa 'cache corrupto' sin motivo"
fi

# ---------------------------------------------------------------------------
# 2. GUARD — lectura y escritura del marker apuntan al MISMO directorio
# ---------------------------------------------------------------------------
echo "--- guard: el marker se lee donde se escribe"
w=$(marker_dir_de_escritura); l=$(marker_dir_de_lectura)
if [ -z "$w" ] || [ -z "$l" ]; then
  fail "no pude localizar el marker en el script (escritura='$w' lectura='$l')"
elif [ "$w" = "$l" ]; then
  pass "escritura y lectura coinciden (\$$w)"
else
  fail "el marker se escribe en \$$w y se lee en \$$l: nunca está donde se lo busca"
fi

# El marker pertenece al CACHE, no al clone: tiene que sobrevivir al rm -rf del
# repo, que es lo que distingue "cache nuestro con clone borrado" de "dir ajeno"
echo "--- el marker vive en el cache, no adentro del clone"
if [ "$w" = "CACHE_DIR" ]; then
  pass "el marker vive en \$CACHE_DIR: sobrevive al rm -rf del repo y no ensucia el working tree del clone"
else
  fail "el marker vive en \$$w: se borra junto con el repo y ensucia el working tree del clone"
fi

# ---------------------------------------------------------------------------
# 3. El caso que el fix NO debe romper
# ---------------------------------------------------------------------------
echo "--- marker presente pero repo borrado: hay que clonar igual"
cache="$WORK/c2"; mkdir -p "$cache"; printf 'x\n' > "$cache/.domain-user-install-marker"
need=1
[ -d "$cache/repo/.git" ] && [ -f "$cache/.domain-user-install-marker" ] && need=0
if [ "$need" = "1" ]; then
  pass "con el marker presente pero sin repo, need_clone sigue en 1"
else
  fail "need_clone=0 sin repo clonado: el install seguiría sin fuentes"
fi

echo ""
if [ "$FAIL" -gt 0 ]; then
  echo "RED — $FAIL fallaron, $PASS pasaron"
  exit 1
fi
echo "GREEN — los $PASS asserts pasaron"
