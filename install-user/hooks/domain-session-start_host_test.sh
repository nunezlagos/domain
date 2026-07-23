#!/usr/bin/env bash
# Test del filtro de host (DOMAINSERV-92) del bloque python de
# domain-session-start.sh: host=cli filtra skills/policies orca-*;
# host=orca las mantiene. Extrae el bloque python del hook y lo ejecuta con
# env simulado (mismo mecanismo que el hook real).
set -euo pipefail
cd "$(dirname "$0")"

python3 - <<'PY'
import json, os, sys

skill_env = json.dumps({"result": {"content": [{"type": "text", "text": json.dumps({"skills": [
    {"slug": "orca-worktree-workflow", "scope": "global"},
    {"slug": "commit-message", "scope": "global"},
    {"slug": "vps-deploy-admin", "scope": "project"}]})}]}})
pol_env = json.dumps({"result": {"content": [{"type": "text", "text": json.dumps({"policies": [
    {"slug": "cross-project-context", "kind": "architecture"},
    {"slug": "structured-logging", "kind": "observability"}]})}]}})

block = open("domain-session-start.sh").read().split("python3 - <<'PYEOF'")[1].split("PYEOF")[0]

def render(host):
    os.environ.update(HOOK_HOST=host, HOOK_SKILL_OUT=skill_env, HOOK_POLICY_OUT=pol_env,
                      HOOK_CTX_MAX_BYTES="12000", HOOK_VPS_URL="x", HOOK_MEM_SLUG="s",
                      HOOK_BOOTSTRAP_OUT="", HOOK_MEM_OUT="", HOOK_SOURCE="startup")
    ns = {}
    exec(compile(block, "block", "exec"), ns)
    # tomar solo las líneas skills:/policies: del bloque de skills&policies
    b = ns["_skills_policies_block"]()
    return {ln.split(":")[0]: ln for ln in b.splitlines() if ln.startswith(("skills:", "policies:"))}

cli = render("cli")
assert "orca-worktree-workflow" not in cli["skills"], "cli debe filtrar skill orca-*"
assert "cross-project-context" not in cli["policies"], "cli debe filtrar policy orca"
assert "commit-message" in cli["skills"] and "vps-deploy-admin" in cli["skills"], "cli conserva no-orca"

orca = render("orca")
assert "orca-worktree-workflow" in orca["skills"], "orca conserva skill orca-*"
assert "cross-project-context" in orca["policies"], "orca conserva policy orca"

print("PASS: host=cli filtra orca-*; host=orca las conserva")
PY
