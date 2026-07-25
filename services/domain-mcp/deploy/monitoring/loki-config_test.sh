#!/usr/bin/env bash
# DOMAINSERV-128: valida loki-config.yml contra el binario REAL de la versión
# pineada en docker-compose.yml, vía docker. Sin mocks.
#
# OJO: server.log_level=warn SUPRIME el "config is valid" de -verify-config, así
# que con una config buena el comando no imprime NADA y sale 0. Todo se assertea
# por EXIT CODE; assertear por texto daría falso negativo.
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
CONFIG="$SCRIPT_DIR/loki-config.yml"
COMPOSE="$SCRIPT_DIR/docker-compose.yml"
SECURITY_CONTAINERS=(domain-caddy domain-honeypot domain-crowdsec)

TMPDIR_TEST="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_TEST"' EXIT

failures=0

pass() { printf 'PASS  %s\n' "$1"; }
fail() {
	printf 'FAIL  %s\n      %s\n' "$1" "$2" >&2
	failures=$((failures + 1))
}

# lee el tag del compose para que el test no se desalinee del deploy real
loki_image() {
	grep -oE 'grafana/loki:[0-9]+\.[0-9]+\.[0-9]+' "$COMPOSE" | head -1
}

# corre -verify-config sobre el archivo dado y devuelve su exit code
verify_config() {
	docker run --rm --entrypoint /usr/bin/loki \
		-v "$1:/etc/loki/loki-config.yml:ro" "$IMAGE" \
		-config.file=/etc/loki/loki-config.yml -verify-config
}

# extrae la retención general en horas
general_hours() {
	grep -oE '^[[:space:]]*retention_period:[[:space:]]*[0-9]+h' "$CONFIG" |
		head -1 | grep -oE '[0-9]+'
}

# extrae el period del primer retention_stream (recorta desde esa clave para no
# capturar el period del index de schema_config, que aparece antes)
stream_hours() {
	sed -n '/retention_stream:/,$p' "$CONFIG" |
		grep -oE '^[[:space:]]*period:[[:space:]]*[0-9]+h' |
		head -1 | grep -oE '[0-9]+'
}

test_real_config_is_valid() {
	local name="real_config_verify_config_exit_zero" out
	if out="$(verify_config "$CONFIG" 2>&1)"; then
		pass "$name"
	else
		fail "$name" "-verify-config rechazó loki-config.yml: $out"
	fi
}

test_broken_selector_is_rejected() {
	local name="broken_logql_selector_verify_config_exit_nonzero"
	local bad="$TMPDIR_TEST/broken-selector.yml"
	# control negativo: llave de cierre faltante en el selector LogQL
	sed 's|selector: .*|selector: '"'"'{container="domain-caddy"'"'"'|' \
		"$CONFIG" >"$bad"
	if verify_config "$bad" >/dev/null 2>&1; then
		fail "$name" "-verify-config ACEPTÓ un selector LogQL roto"
	else
		pass "$name"
	fi
}

test_security_streams_retention_is_longer() {
	local name="security_streams_retention_longer_than_general"
	local general stream
	general="$(general_hours)"
	stream="$(stream_hours)"
	if [ -z "$general" ] || [ -z "$stream" ]; then
		fail "$name" "no pude leer los periodos (general='$general' stream='$stream')"
	elif [ "$stream" -le "$general" ]; then
		fail "$name" "el stream de seguridad (${stream}h) no supera al general (${general}h)"
	else
		pass "$name"
	fi
}

test_all_security_containers_are_covered() {
	local name="security_containers_present_in_retention_stream"
	local block missing=()
	block="$(sed -n '/retention_stream:/,/^  [a-z]/p' "$CONFIG")"
	for c in "${SECURITY_CONTAINERS[@]}"; do
		grep -q "$c" <<<"$block" || missing+=("$c")
	done
	if [ "${#missing[@]}" -gt 0 ]; then
		fail "$name" "sin cobertura de retención: ${missing[*]}"
	else
		pass "$name"
	fi
}

test_compactor_retention_stays_enabled() {
	local name="compactor_retention_enabled_and_delete_store_set"
	local missing=()
	grep -qE '^[[:space:]]*retention_enabled:[[:space:]]*true' "$CONFIG" ||
		missing+=("retention_enabled: true")
	grep -qE '^[[:space:]]*delete_request_store:[[:space:]]*filesystem' "$CONFIG" ||
		missing+=("delete_request_store: filesystem")
	if [ "${#missing[@]}" -gt 0 ]; then
		fail "$name" "sin esto el compactor NO borra nada: ${missing[*]}"
	else
		pass "$name"
	fi
}

main() {
	command -v docker >/dev/null || {
		echo "SKIP: docker no disponible" >&2
		exit 1
	}
	IMAGE="$(loki_image)"
	[ -n "$IMAGE" ] || {
		echo "FAIL: no encontré grafana/loki:<version> en $COMPOSE" >&2
		exit 1
	}
	echo "loki image (pineada en el compose): $IMAGE"
	test_real_config_is_valid
	test_broken_selector_is_rejected
	test_security_streams_retention_is_longer
	test_all_security_containers_are_covered
	test_compactor_retention_stays_enabled
	if [ "$failures" -gt 0 ]; then
		printf '\n%d test(s) fallaron\n' "$failures" >&2
		exit 1
	fi
	printf '\n5 test(s) OK\n'
}

main "$@"
