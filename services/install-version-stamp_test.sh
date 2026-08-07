#!/usr/bin/env bash
# Guard del sello de versión que install.sh escribe en el .env (issue-57.1).
#
# LA REGRESIÓN QUE MOTIVA ESTA SUITE, medida el 2026-08-07: el Dockerfile declaraba
# `ARG VERSION=dev` y el compose pasaba `VERSION: ${VERSION:-dev}`, pero NADIE definía
# VERSION en ningún lado. El server se desplegaba con Version="dev", el bootstrap devolvía
# server.version="dev", EsVersionDeRelease lo rechazaba, y el aviso de actualización del
# cliente quedaba inerte — con todo el código de REQ-57 fase 0 en su lugar y en verde.
#
# Es la policy guards-deben-ejecutarse en su forma más cara: una cadena completa donde el
# único eslabón faltante no producía ningún error, solo silencio.
set -uo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
INSTALL_SH="$SCRIPT_DIR/install.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

FALLOS=0
ok()   { printf '  ✓ %s\n' "$1"; }
fail() { printf '  ✗ %s\n' "$1"; FALLOS=$((FALLOS + 1)); }

[[ -f "$INSTALL_SH" ]] || { echo "no encuentro install.sh en $SCRIPT_DIR"; exit 1; }

echo "== el .env recibe las tres variables que el compose consume =="

# El compose las lee del .env por nombre; si install.sh deja de escribir alguna, el build
# cae al default del Dockerfile y el server vuelve a decir que es "dev".
for var in VERSION COMMIT BUILD_TIME; do
  if grep -qE "env_set[[:space:]]+\"?${var}\"?" "$INSTALL_SH"; then
    ok "install.sh escribe $var en el .env"
  else
    fail "install.sh NO escribe $var: el build cae al default del Dockerfile y el server se declara 'dev'"
  fi
done

echo "== el update del repo trae los tags =="

# `git fetch origin <rama>` sin --tags puede no traer un tag nuevo, y sin el tag
# `git describe --exact-match` no lo encuentra: el deploy se sellaría como 'dev' aunque el
# tag exista en el remoto. Es el modo de falla más silencioso de toda la cadena.
if grep -qE 'git fetch[^&|]*--tags' "$INSTALL_SH"; then
  ok "el fetch del repo incluye --tags"
else
  fail "el fetch NO trae tags: un tag publicado no llegaría al VPS y el sello quedaría en 'dev'"
fi

echo "== la derivación de la versión se comporta como debe (ejecutada de verdad) =="

# No se puede correr install.sh acá (instala paquetes y necesita root), así que se ejercita
# la MISMA expresión contra repos git reales. Si la expresión del script cambia, este bloque
# hay que actualizarlo — a propósito: obliga a pensar qué se cambió.
derivar_version() { git -C "$1" describe --tags --exact-match 2>/dev/null || echo dev; }

REPO="$WORK/repo"
git init -q "$REPO" 2>/dev/null
git -C "$REPO" config user.email t@t.cl
git -C "$REPO" config user.name t
: > "$REPO/a"; git -C "$REPO" add a; git -C "$REPO" commit -qm "primero"

got=$(derivar_version "$REPO")
if [[ "$got" == "dev" ]]; then
  ok "sin ningún tag → 'dev' (no finge ser una release)"
else
  fail "sin tag devolvió '$got'; debe ser 'dev' o el cliente comparará contra una versión inexistente"
fi

git -C "$REPO" tag v1.2.3
got=$(derivar_version "$REPO")
if [[ "$got" == "v1.2.3" ]]; then
  ok "con el tag exacto en HEAD → 'v1.2.3'"
else
  fail "con tag exacto devolvió '$got', esperaba 'v1.2.3'"
fi

: > "$REPO/b"; git -C "$REPO" add b; git -C "$REPO" commit -qm "segundo"
got=$(derivar_version "$REPO")
if [[ "$got" == "dev" ]]; then
  ok "un commit DESPUÉS del tag → 'dev', no 'v1.2.3'"
else
  fail "commit posterior al tag devolvió '$got': un deploy de código no publicado se haría pasar por la release $got"
fi

echo "== el .env no queda con el sello de un deploy anterior =="

# Las credenciales se PRESERVAN entre deploys a propósito; la versión NO puede: cada deploy
# despliega un commit distinto. Si el sello se escribiera solo cuando falta, el .env
# conservaría la versión del primer deploy para siempre.
if grep -qE "env_set[[:space:]]+\"?VERSION\"?" "$INSTALL_SH" && \
   ! grep -B4 -E "env_set[[:space:]]+\"?VERSION\"?" "$INSTALL_SH" | grep -qE '^\s*(if|\[\[).*(-z|CHANGE_ME|EXISTING)'; then
  ok "el sello se sobrescribe en cada deploy, no se preserva"
else
  fail "el sello parece preservarse como una credencial: el .env quedaría con la versión de un deploy viejo"
fi

echo
if [[ $FALLOS -gt 0 ]]; then
  echo "FALLARON $FALLOS chequeo(s)"
  exit 1
fi
echo "OK — el sello de versión del deploy está cableado"
