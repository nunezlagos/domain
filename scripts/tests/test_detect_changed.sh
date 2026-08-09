#!/usr/bin/env bash
# scripts/tests/test_detect_changed.sh
#
# DOMANSERV-268 — detect_changed_services decide qué se rebuildea en cada deploy, y hasta
# hoy no tenía suite propia: la única cobertura era indirecta, por test_deploy.sh, que
# ejercita el flujo entero en dry-run pero nunca la función por casos de directorio.
#
# Por eso este bug estaba a la vista y sin un test que lo nombrara: un cambio en
# services/scripts/ —scripts del HOST, sin container que reconstruir— devolvía "all", y
# `make restart SVC=all` es `down && build && up` de los 17 containers. Downtime completo,
# automático, diez minutos después de publicar un tag.
#
# La causa es que `return 1` significaba DOS cosas a la vez: "no sé qué es esto" (servicio
# nuevo sin mapear, donde rebuildear todo es lo prudente) y "esto no es un servicio". El
# fix separa la segunda; la primera NO se toca, y el caso del dir desconocido es uno de
# los tests justamente para que nadie la degrade por error.
#
# Método: un repo git de verdad en un tmpdir, con commits reales, porque la función opera
# sobre `git diff --name-only` y stubearlo probaría el stub.
#
# Uso: bash scripts/tests/test_detect_changed.sh

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LIB="$REPO_ROOT/scripts/lib/detect_changed.sh"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

failed=0
pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; failed=$((failed + 1)); }

assert_eq() {
  local label="$1" esperado="$2" actual="$3"
  if [[ "$esperado" == "$actual" ]]; then pass "$label"; else
    fail "$label — esperado='$esperado' actual='$actual'"
  fi
}

if [[ ! -f "$LIB" ]]; then
  echo "RED: $LIB no existe" >&2
  exit 1
fi
# shellcheck source=/dev/null
source "$LIB"

# Repo real con dos commits: el base y otro que toca los paths del caso. La función corre
# `git diff --name-only prev..new`, así que un stub de git probaría el stub y no la regla.
detectar_para() {
  local nombre="$1"; shift
  local repo="$WORK/$nombre"
  mkdir -p "$repo"
  (
    cd "$repo"
    git init -q -b main
    git config user.email t@t; git config user.name t
    echo base > README.md
    git add -A; git commit -q -m base

    local ruta
    for ruta in "$@"; do
      mkdir -p "$(dirname "$ruta")"
      echo cambio > "$ruta"
    done
    git add -A; git commit -q -m cambio
  ) >/dev/null 2>&1
  ( cd "$repo" && detect_changed_services HEAD~1 HEAD )
}

# --- los dos casos que motivan el change ---

test_host_only_no_pide_rebuild() {
  local T="TestDetect_SoloScriptsDelHost_NoPideRebuild"
  # es el caso REAL que lo destapó: la extensión del aviso tocaba este archivo y nada más
  assert_eq "$T — services/scripts/ no rebuildea nada" \
    "" "$(detectar_para scripts services/scripts/auto-deploy-alert.sh)"
  assert_eq "$T — services/systemd/ tampoco" \
    "" "$(detectar_para systemd services/systemd/domain-auto-deploy.service)"
}

# --- los que NO deben cambiar, y por qué cada uno importa ---

test_servicio_conocido_sigue_mapeando() {
  local T="TestDetect_ServicioConocido_SigueMapeando"
  assert_eq "$T — domain-mcp -> mcp" \
    "mcp" "$(detectar_para mcp services/domain-mcp/internal/x.go)"
  assert_eq "$T — domain-admin -> admin" \
    "admin" "$(detectar_para admin services/domain-admin/app/y.py)"
}

test_makefile_y_compose_siguen_pidiendo_todo() {
  local T="TestDetect_MakefileOCompose_SiguenPidiendoTodo"
  # tocar el Makefile o un compose puede cambiar CÓMO se construye cualquier servicio
  assert_eq "$T — services/Makefile -> all" \
    "all" "$(detectar_para makefile services/Makefile)"
  assert_eq "$T — un docker-compose -> all" \
    "all" "$(detectar_para compose services/domain-mcp/docker-compose.yml)"
}

# ESTE es el que protege la regla defensiva. Si alguien "arregla" el bug degradando todo
# lo no mapeado a noop, un servicio nuevo NUNCA se desplegaría — y ese fallo es silencioso,
# mientras que un rebuild de más es ruidoso y se nota. El sentido de la asimetría es ese.
test_dir_desconocido_sigue_pidiendo_todo() {
  local T="TestDetect_DirDesconocido_SiguePidiendoTodo"
  assert_eq "$T — un servicio nuevo sin mapear -> all" \
    "all" "$(detectar_para nuevo services/domain-loquesea/main.go)"
}

# El caso donde un fix apurado rompe algo: si el host-only cortocircuita el recorrido con
# un return temprano, el servicio que cambió en el mismo rango se queda sin desplegar.
test_mezcla_no_suprime_al_servicio_real() {
  local T="TestDetect_HostOnlyMasServicio_NoSuprimeAlServicio"
  assert_eq "$T — scripts + domain-mcp -> mcp (ni vacío ni all)" \
    "mcp" "$(detectar_para mezcla services/scripts/x.sh services/domain-mcp/y.go)"
  assert_eq "$T — scripts + un dir desconocido -> all (la duda sigue mandando)" \
    "all" "$(detectar_para mezcla-desconocido services/scripts/x.sh services/domain-nuevo/z.go)"
}

test_sin_cambios_bajo_services_no_despliega() {
  local T="TestDetect_SinCambiosBajoServices_NoDespliega"
  assert_eq "$T — un cambio fuera de services/ no rebuildea nada" \
    "" "$(detectar_para fuera scripts/lib/detect_changed.sh)"
}

test_host_only_no_pide_rebuild
test_servicio_conocido_sigue_mapeando
test_makefile_y_compose_siguen_pidiendo_todo
test_dir_desconocido_sigue_pidiendo_todo
test_mezcla_no_suprime_al_servicio_real
test_sin_cambios_bajo_services_no_despliega

if (( failed > 0 )); then
  echo "RED — $failed asserts fallaron"
  exit 1
fi

echo "GREEN — todos los tests pasaron"
exit 0
