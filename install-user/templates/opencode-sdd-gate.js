// domain-sdd-gate.js — DOMAINSERV-100
// Gate SDD + commit-gate para OpenCode a nivel plugin, paridad con el
// domain-pre-edit.sh + domain-post-orchestrate.sh de Claude Code. Complementa el
// git-guard (69b, archivo aparte).
//
// Mecánica (espeja Claude Code):
//  - tool.execute.before: si el tool edita código y NO hay flow-token válido para
//    el sessionID (validado contra el server) → throw (deny). Con allowed_paths
//    (DOMAINSERV-110 batch-mode) scopea la edición por path. git commit sin marker
//    tests-ok fresco → deny (DOMAINSERV-74).
//  - tool.execute.after de domain_orchestrate → mintea el flow-token vía
//    domain_flow_grant_token y escribe ~/.local/state/domain/flow-<sessionID>
//    (mismo formato que el hook de Claude: token\texpires\tmode).
//
// NO validado en runtime OpenCode todavía (falta entorno): los tool ids
// ("edit"/"write"/"patch"/"bash"), el arg del path (filePath) y el shape del
// output se confirman al correrlo. sessionID sí está garantizado por la API
// (tool.execute.before input.sessionID, verificado con context7 /anomalyco/opencode).

import { homedir } from "os"
import { join } from "path"
import { readFileSync, writeFileSync, mkdirSync, statSync } from "fs"

const STATE_DIR = join(homedir(), ".local", "state", "domain")
const EDIT_TOOLS = new Set(["edit", "write", "patch"])
const SRC_EXT =
  /\.(go|py|ts|tsx|js|jsx|sql|sh|bash|rs|java|kt|php|rb|c|cc|cpp|h|hpp|vue|svelte|toml|tf|hcl|env|gradle|cs|scala|swift|proto|lua)\b/
const CODE_EXT =
  /\.(go|py|ts|tsx|js|jsx|sql|sh|bash|rs|java|kt|php|rb|c|cc|cpp|h|hpp|vue|svelte|yaml|yml|json|toml|tf|hcl|env|xml|gradle|cs|scala|swift|proto|lua)\b/

function resolveEnv() {
  let vpsUrl = process.env.DOMAIN_VPS_URL || ""
  let apiKey = process.env.DOMAIN_API_KEY || ""
  if (vpsUrl && apiKey) return { vpsUrl, apiKey }
  const files = [
    join(homedir(), ".config", "domain", "install.env"),
    join(homedir(), ".claude", ".env"),
    join(homedir(), ".config", "opencode", ".env"),
  ]
  for (const f of files) {
    let content
    try {
      content = readFileSync(f, "utf8")
    } catch {
      continue
    }
    for (const line of content.split("\n")) {
      const i = line.indexOf("=")
      if (i < 0) continue
      const k = line.slice(0, i).trim()
      const v = line.slice(i + 1).trim().replace(/^["']|["']$/g, "")
      if (k === "DOMAIN_VPS_URL" && !vpsUrl) vpsUrl = v
      if ((k === "DOMAIN_MCP_API_KEY" || k === "DOMAIN_API_KEY") && !apiKey) apiKey = v
    }
  }
  return { vpsUrl, apiKey }
}

function markerPath(sessionID) {
  return join(STATE_DIR, "flow-" + sessionID)
}

function readMarker(sessionID) {
  try {
    const first = readFileSync(markerPath(sessionID), "utf8").split("\n")[0]
    const [token, expires, mode] = first.split("\t")
    return { token, expires, mode }
  } catch {
    return null
  }
}

function freshMarker(p, maxMinutes) {
  try {
    return Date.now() - statSync(p).mtimeMs < maxMinutes * 60000
  } catch {
    return false
  }
}

async function callTool(vpsUrl, apiKey, name, args) {
  const res = await fetch(vpsUrl.replace(/\/$/, "") + "/mcp", {
    method: "POST",
    headers: {
      Authorization: "Bearer " + apiKey,
      "Content-Type": "application/json",
      Accept: "application/json, text/event-stream",
    },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "tools/call", params: { name, arguments: args } }),
    signal: AbortSignal.timeout(6000),
  })
  const text = await res.text()
  const m = text.match(/\{[\s\S]*\}/) // tolera SSE: extrae el objeto JSON
  return m ? JSON.parse(m[0]) : null
}

function toolTextBody(resp) {
  try {
    for (const c of resp.result.content) {
      if (c.type === "text") return JSON.parse(c.text)
    }
  } catch {}
  return null
}

function isCodeEditBash(cmd) {
  if (/\bsed\s+(-\w*\s+)*-i/.test(cmd)) return true
  if (/\bperl\s+(-\w*\s+)*-i/.test(cmd)) return true
  if (new RegExp(">>?\\s*\\S*" + SRC_EXT.source).test(cmd)) return true
  if (/\btee\s+(-a\s+)?\S*/.test(cmd) && CODE_EXT.test(cmd)) return true
  if (/\bgit\s+apply\b/.test(cmd)) return true
  return false
}

function globToRegex(g) {
  const esc = g.replace(/[.+^${}()|[\]\\]/g, "\\$&").replace(/\*/g, ".*").replace(/\?/g, ".")
  return new RegExp("^" + esc + "$")
}

function pathAllowed(fp, globs, directory) {
  const cands = [fp]
  if (directory && fp.startsWith(directory + "/")) cands.push(fp.slice(directory.length + 1))
  for (const g of globs) {
    const re = globToRegex(g)
    for (const c of cands) if (re.test(c)) return true
  }
  return false
}

export const DomainSddGate = async ({ directory }) => {
  const { vpsUrl, apiKey } = resolveEnv()
  return {
    "tool.execute.before": async (input, output) => {
      const tool = input.tool
      const sessionID = input.sessionID || ""
      const args = (output && output.args) || {}
      const cmd = tool === "bash" ? String(args.command || "") : ""

      // COMMIT-GATE (DOMAINSERV-74): git commit exige marker tests-ok fresco (<30min).
      if (tool === "bash" && /\bgit\s+commit\b/.test(cmd)) {
        if (!freshMarker(join(STATE_DIR, "tests-ok-" + sessionID), 30)) {
          throw new Error(
            "domain commit-gate (DOMAINSERV-74): no hay corrida de tests que cubra el estado actual. " +
              "Corré la suite de tests antes de commitear.",
          )
        }
        return
      }

      const editsCode = EDIT_TOOLS.has(tool) || (tool === "bash" && isCodeEditBash(cmd))
      if (!editsCode) return

      const filePath = args.filePath || args.path || ""
      const mk = readMarker(sessionID)
      if (mk && mk.token && vpsUrl && apiKey) {
        let body = null
        try {
          const resp = await callTool(vpsUrl, apiKey, "domain_flow_validate_token", {
            token: mk.token,
            session_id: sessionID,
          })
          body = toolTextBody(resp)
        } catch {}
        if (body && body.valid) {
          const allowed = body.allowed_paths || []
          if (!allowed.length) return // flow normal: sin restricción de path
          if (!filePath) return // bash-edit sin path claro: token válido alcanza
          if (pathAllowed(filePath, allowed, directory)) return
          throw new Error(
            "domain batch-mode (DOMAINSERV-110): el path '" +
              filePath +
              "' está fuera de la allowlist del flow activo (" +
              JSON.stringify(allowed) +
              "). Editá dentro del scope o abrí un flow para este path.",
          )
        }
      }
      throw new Error(
        "domain (issue-54.7): edición de código SIN flow SDD activo. Ejecutá domain_orchestrate " +
          "(mode express para cambios ≤10 líneas single-file, lite para cambios chicos) ANTES de editar.",
      )
    },
    "tool.execute.after": async (input, output) => {
      // GRANT: al terminar domain_orchestrate, mintear el flow-token y escribir el marker.
      if (input.tool !== "domain_orchestrate") return
      if (!vpsUrl || !apiKey) return
      let flowRunID = ""
      let mode = ""
      try {
        const body = JSON.parse(output.output)
        flowRunID = body.flow_run_id || body.id || ""
        mode = body.mode || ""
      } catch {}
      if (!flowRunID) return
      const sessionID = input.sessionID || ""
      try {
        const resp = await callTool(vpsUrl, apiKey, "domain_flow_grant_token", {
          flow_run_id: flowRunID,
          session_id: sessionID,
        })
        const body = toolTextBody(resp)
        if (!body || !body.token) return
        const expires = new Date(Date.now() + (body.expires_in || 1800) * 1000).toISOString()
        mkdirSync(STATE_DIR, { recursive: true, mode: 0o700 })
        writeFileSync(markerPath(sessionID), body.token + "\t" + expires + "\t" + mode + "\n", { mode: 0o600 })
      } catch {}
    },
  }
}
