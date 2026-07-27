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

def render(host, skills=None, policies=None, max_bytes="12000"):
    os.environ.update(HOOK_HOST=host,
                      HOOK_SKILL_OUT=skill_env if skills is None else skills,
                      HOOK_POLICY_OUT=pol_env if policies is None else policies,
                      HOOK_CTX_MAX_BYTES=max_bytes, HOOK_VPS_URL="x", HOOK_MEM_SLUG="s",
                      HOOK_BOOTSTRAP_OUT="", HOOK_MEM_OUT="", HOOK_SOURCE="startup")
    ns = {}
    exec(compile(block, "block", "exec"), ns)
    b = ns["_skills_policies_block"]()
    # el bloque crudo se devuelve aparte: los asserts del token miran la primera
    # línea, que las claves skills:/policies: no cubren
    out = {ln.split(":")[0]: ln for ln in b.splitlines() if ln.startswith(("skills:", "policies:"))}
    out["_raw"] = b
    out["_head"] = b.splitlines()[0]
    out["_cap"] = ns["_cap"](b, int(int(max_bytes) * 0.20))
    return out

cli = render("cli")
assert "orca-worktree-workflow" not in cli["skills"], "cli debe filtrar skill orca-*"
assert "cross-project-context" not in cli["policies"], "cli debe filtrar policy orca"
assert "commit-message" in cli["skills"] and "vps-deploy-admin" in cli["skills"], "cli conserva no-orca"

orca = render("orca")
assert "orca-worktree-workflow" in orca["skills"], "orca conserva skill orca-*"
assert "cross-project-context" in orca["policies"], "orca conserva policy orca"

# DOMAINSERV-179 modo ok: el token va en el header y los counts son POST-filtro
# orca, si no el número del header contradice R8
assert "skpol=ok" in cli["_head"], "camino feliz debe marcar skpol=ok"
assert "P_sk=1" in cli["_head"] and "G_sk=1" in cli["_head"], "counts de skills post-filtro en el header"
assert "P_pol=1" in cli["_head"], "counts de policies post-filtro en el header"

# modo degraded: el curl no devolvió nada. Hoy el aviso va solo a stderr, que el
# agente nunca ve — el token es lo que lo hace visible
deg = render("cli", skills="")
assert "skpol=degraded" in deg["_head"], "sin respuesta del MCP debe marcar skpol=degraded"
assert "no disponible" in deg["skills"], "se conserva el detalle humano de la línea"

# modo truncado: _cap corta por líneas completas desde arriba, así que el header
# con el token y los counts sobrevive aunque se dropeen los slugs
tight = render("cli", max_bytes="1200")
assert "skpol=ok" in tight["_cap"], "el token debe sobrevivir a _cap"
assert "P_sk=1" in tight["_cap"], "los counts deben sobrevivir a _cap"

print("PASS: host=cli filtra orca-*; host=orca las conserva; skpol ok/degraded y counts sobreviven _cap")
PY
