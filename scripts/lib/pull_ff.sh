#!/usr/bin/env bash
# scripts/lib/pull_ff.sh
#
# Wrapper de git pull --ff-only para auto-deploy (HU 38.12). Opera sobre
# el git repo del CWD (el runner configura origin antes de invocarlo).
# NUNCA hace rollback: si diverge, el error es del operador en el VPS y
# deploy.sh debe propagarlo sin deshacer nada.
#
# Contrato:
#   pull_ff
#     -> exit 0 si fast-forward OK.
#     -> exit 1 + stderr explicativo si divergencia non-ff (rollback NO).

pull_ff() {
  local err_file
  err_file="$(mktemp)"
  if ! git pull --ff-only 2>"$err_file" 1>/dev/null; then
    printf 'pull_ff: divergencia non-fast-forward contra origin/main.\n' >&2
    printf 'Resolvelo manualmente: git fetch origin && git rebase origin/main\n' >&2
    cat "$err_file" >&2
    rm -f "$err_file"
    return 1
  fi
  rm -f "$err_file"
}
