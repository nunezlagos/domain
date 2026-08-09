#!/usr/bin/env bash
# scripts/lib/orchestrator.sh
#
# Funciones puras del orquestador deploy.sh (HU 38.12). Pensadas para
# source-arse. Proveen log_phase, validate_env_no_laxo y la decision
# should_rollback. Las fases (fetch/detect/build/restart/verify) viven
# en deploy.sh porque dependen de make y del estado del runner.

log_phase() {
  local phase="$1" ts linea
  ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf -v linea '[%s] %s' "$ts" "$phase"

  # stderr primero, porque es el destino que NO puede fallar: es lo que systemd captura
  printf '%s\n' "$linea" >&2
  # el archivo es accesorio y su fallo no puede tumbar el deploy. Con `| tee` bajo
  # pipefail, un log no escribible mataba el script en su primera línea: el auto-deploy
  # corre como root y dejó /opt/services/.deploy.log en root:root, así que el redeploy
  # manual de sysadmin salía rc=1 sin llegar a ninguna fase
  printf '%s\n' "$linea" >> "${LOG_FILE:-.deploy.log}" 2>/dev/null || true
}

# El criterio son los BITS de group y other, no `-w`. `-w` responde "¿puede escribirlo
# quien pregunta?" y bajo root eso es siempre sí, porque root ignora los permisos: el
# guard abortaba en las 61 corridas del auto-deploy sin mirar nada. Además contradecía al
# resto del sistema, que exige el .env en 600 — escribible por su dueño y correcto así.
validate_env_no_laxo() {
  local env_file="${DEPLOY_ENV_FILE:-${RUNTIME_DIR:-.}/services/.env}"
  [[ -f "$env_file" ]] || { log_phase "validate: .env ausente ($env_file)"; return 1; }

  # -L dereferencia: en producción services/.env es un symlink a ../.env, y el modo de un
  # symlink es SIEMPRE 777 en Linux, sin relación con lo que apunta. Sin -L el check
  # clasificaba como laxo un .env que está en 600. Lo que importa es el archivo que
  # guarda los secretos, no el enlace que lleva hasta él.
  local modo; modo="$(stat -Lc '%a' "$env_file" 2>/dev/null)" \
    || { log_phase "validate: no se pudo leer el modo de $env_file"; return 1; }
  # los dos últimos dígitos son group y other; 2,3,6,7 son los que llevan el bit de escritura
  if [[ "${modo: -2}" =~ [2367] ]]; then
    log_phase "validate: .env laxo (modo $modo, escribible por group u other) — abort"
    return 1
  fi

  grep -q '^DOMAIN_FIELD_ENC_KEY=' "$env_file" || { log_phase "validate: DOMAIN_FIELD_ENC_KEY ausente"; return 1; }
  return 0
}

should_rollback() {
  [[ -n "${NOOP:-}" ]] && return 1
  return 0
}
