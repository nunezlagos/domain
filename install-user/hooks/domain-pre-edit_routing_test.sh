#!/usr/bin/env bash
# Test del dispatch v1-legacy / v2-HMAC del gate pre-edit (DOMAINSERV-107).
#
# Regresión: un marker legacy "<timestamp>\t<flow_run_id>" NO debe enrutarse
# como token v2 (field1=timestamp matcheaba el glob de token). El ruteo correcto
# es por field2: si es UUID → legacy (flow_status); si no y field1 no está vacío
# → v2 (validate_token); vacío → deny.
set -u

# route replica EXACTAMENTE la decisión de domain-pre-edit.sh (líneas del
# bloque DOMAINSERV-107). Si cambia el hook, actualizar acá.
route() {
  local field1="$1" field2="$2"
  if printf '%s' "$field2" | grep -qE '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$'; then
    echo "legacy"
  elif [ -n "$field1" ]; then
    echo "v2"
  else
    echo "deny"
  fi
}

fail=0
assert() {
  local got="$1" want="$2" name="$3"
  if [ "$got" = "$want" ]; then
    echo "  PASS $name ($got)"
  else
    echo "  FAIL $name: got=$got want=$want"; fail=1
  fi
}

echo "== dispatch pre-edit (DOMAINSERV-107) =="
# marker legacy: field1=timestamp, field2=flow_run_id (UUID)
assert "$(route '2026-07-23T16:39:03-04:00' '98ed3edb-b142-4868-b5e3-028f841e7ac1')" "legacy" "legacy_marker→flow_status"
# marker v2: field1=token HMAC, field2=expires_at (ISO, NO uuid)
assert "$(route 'eyJhbGciOiJIUzI1NiJ9.payload.sig' '2026-07-23T17:09:03-04:00')" "v2" "v2_marker→validate_token"
# marker corrupto/vacío
assert "$(route '' '')" "deny" "vacío→deny"
# UUID mayúsculas también es legacy
assert "$(route 'ts' 'ABCDEF12-1234-5678-9ABC-DEF012345678')" "legacy" "uuid_mayus→legacy"

[ "$fail" = 0 ] && echo "OK todos los casos" || { echo "FALLÓ"; exit 1; }
