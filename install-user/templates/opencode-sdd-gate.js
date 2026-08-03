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
import { readFileSync, writeFileSync, mkdirSync, statSync, unlinkSync } from "fs"
import { execFileSync } from "child_process"

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

// ─── GUARD DESTRUCTIVO (DOMAINSERV-222) ───────────────────────────────────────
// Espejo EXACTO del bloque (A2) de domain-pre-edit.sh. La divergencia es el riesgo
// real acá: este mismo archivo nació con el commit-gate reducido a un chequeo de
// mtime mientras el de bash validaba el hash del árbol. hook_destructive_guard_test.go
// corre el corpus contra las DOS implementaciones y falla si una decide distinto.
//
// El alcance NO mira el FLAG, mira el RADIO DE DAÑO del objetivo. isDestructiveCommand
// devuelve un MOTIVO (radio | radio-indecidible | sensible | sql | sql-opaco |
// automarker) o "" — el motivo TAMBIÉN se compara en el test de paridad, así que no
// alcanza con que las dos implementaciones disparen: tienen que hacerlo por lo MISMO.
//
// EL PRINCIPIO (invertido en la tercera ronda): si el objetivo de un rm recursivo no es un
// literal que se pueda resolver con CERTEZA, se ESCALA — no se adivina. Antes el guard
// resolvía lo que podía y, cuando no podía, concluía "no es catastrófico": fail-OPEN. Esa
// era la causa ÚNICA de tres evasiones que parecían distintas (`rm -rf $PWD # PWD=tmp`,
// `F=-rf; rm $F /`, `rm -rf .g*`).
const PH_A = "\u0001"
const SHELLS = new Set(["sh", "bash", "zsh", "dash", "ksh"])
const SQL_CLIENTES = new Set(["psql", "mysql", "mariadb", "mysqlsh", "sqlite3"])
// Envolturas REMOTAS: el cwd y la raíz del repo del otro lado son DESCONOCIDOS, así que
// ahí el radio se evalúa fail-closed. docker/kubectl solo ejecutan un comando AJENO con
// estos subcomandos: `docker rm ctr` borra un container y no despierta el guard.
// TERCERA RONDA: el match era base(toks[0]) EXACTO contra "docker", así que
// docker-compose (v1, con guión) y podman quedaban afuera y eran evasión directa.
const REMOTAS_SUB = {
  docker: ["exec", "run"],
  "docker-compose": ["exec", "run"],
  podman: ["exec", "run"],
  "podman-compose": ["exec", "run"],
  nerdctl: ["exec", "run"],
  lxc: ["exec"],
  incus: ["exec"],
  kubectl: ["exec"],
  oc: ["exec"],
}
const REMOTAS = new Set(["ssh", "scp"])
// Envolturas LOCALES: el objetivo se resuelve contra el cwd real.
// TERCERA RONDA: eval SIN comillas era una evasión total. `eval "rm -rf ."` disparaba porque
// el comando viajaba en un literal y la recursión lo miraba adentro; `eval rm -rf .` no tiene
// literal ninguno, así que toks[0] quedaba en "eval", posicionesRm no encontraba el rm en la
// posición 0 y NADA se evaluaba — apagaba radio, sensible y sql de un solo golpe. eval es una
// envoltura LOCAL: lo que sigue corre en ESTE shell, con ESTE cwd.
const LOCALES = new Set(["xargs", "find", "eval"])
const WRAPPERS = new Set([
  "sudo", "doas", "command", "env", "time", "timeout", "nohup", "setsid",
  "stdbuf", "exec", "ionice", "nice",
])
// flags de wrapper que CONSUMEN el token siguiente (sudo -u www-data, nice -n 10).
const WRAP_VALOR = new Set([
  "-u", "-g", "-n", "-o", "-k", "-s", "-p", "-C", "--user", "--group", "--signal",
  "--kill-after", "--adjustment", "--class", "--classdata", "--chdir", "--output",
])
// gramática de shell: estos tokens NO son el comando del segmento.
const DESCARTES = new Set([
  "then", "do", "else", "elif", "if", "while", "until", "{", "(", "!", "&&", "||", "|&",
])
// Raíces de app en containers: /app suele ser un BIND MOUNT del repo del host.
const RAICES_APP = ["/app", "/srv", "/repo", "/workspace", "/var/www", "/usr/src/app", "/code"]
const EFIMEROS = ["/tmp", "/var/tmp", "/dev/shm", "/var/cache", "/var/log", "/run"]
// Subdirectorios que se regeneran: borrarlos recursivamente es rutina de desarrollo.
const RUTINA = new Set([
  "node_modules", "dist", "build", "vendor", "target", ".next", ".nuxt", "out",
  "__pycache__", ".pytest_cache", ".mypy_cache", ".venv", "venv", ".cache",
  "tmp", "temp", "logs", "bin", "obj", ".terraform", ".gradle",
])
// Archivos que NO viven en git: borrarlos no se revierte con un checkout.
const SENSIBLES = [
  /^\.env(?:$|[.\-_])/,
  /^id_(?:rsa|dsa|ecdsa|ed25519)(?:$|\.)/,
  /\.(?:key|pem|p12|pfx|jks)(?:$|\.)/,
  /credential/,
  /secret/,
]
// .env.example / secrets.md son PLANTILLA y DOC: van al repo, no son el secreto.
const EJEMPLAR = /\.(?:example|sample|template|dist|tpl|md|rst)$/
// mkdir es el peor de la lista: crea el marker como DIRECTORIO, y el rm -f con el que el
// gate lo consume no borra directorios, así que el bypass quedaba PERMANENTE
const VERBOS_ESCRITURA = new Set(["tee", "cp", "mv", "touch", "ln", "install", "dd", "rsync", "mkdir"])
// Intérpretes que pueden ESCRIBIR el marker desde su propio literal (open(...,"w")).
const INTERPRETES_ESCRITURA = new Set([
  "python", "python2", "python3", "node", "ruby", "perl", "php",
])
// TERCERA RONDA — el mapa de asignaciones NO puede sobrescribir las variables con las que
// el guard MIDE el radio. Envenenarlas era apagar el guard con su propia herramienta:
// `rm -rf $PWD # PWD=tmp` resolvía a ./tmp y pasaba.
const PROTEGIDAS = new Set([
  "PWD", "OLDPWD", "HOME", "CWD", "PROJECT", "PROJECT_ROOT", "REPO", "REPO_ROOT",
  "GIT_ROOT", "ROOT", "WORKSPACE",
])
// Metacaracteres de expansión de PATH. bash los resuelve; el guard los comparaba como
// texto plano, así que `.g*` (= .git .github .gitignore) le parecía un nombre literal.
const GLOB = "*?[]{}"

// normpath sin tocar el filesystem (espeja os.path.normpath para nuestros casos)
function norm(p) {
  if (!p) return ""
  const abs = p.startsWith("/")
  const partes = []
  for (const seg of p.split("/")) {
    if (!seg || seg === ".") continue
    if (seg === "..") {
      if (partes.length && partes[partes.length - 1] !== "..") partes.pop()
      else if (!abs) partes.push("..")
      continue
    }
    partes.push(seg)
  }
  const out = (abs ? "/" : "") + partes.join("/")
  return out || (abs ? "/" : ".")
}

let raizCache = null
function raizGit(cwd) {
  if (raizCache !== null) return raizCache
  raizCache = ""
  try {
    raizCache = norm(
      execFileSync("git", ["-C", cwd, "rev-parse", "--show-toplevel"], {
        encoding: "utf8",
        stdio: ["ignore", "pipe", "ignore"],
        timeout: 3000,
      }).trim(),
    )
  } catch {}
  return raizCache
}

function ctxBase(directory) {
  const cwd = norm(directory || process.cwd())
  return { cwd, raiz: raizGit(cwd), home: norm(homedir()), remoto: false }
}

const base = (tok) => tok.slice(tok.lastIndexOf("/") + 1)

const expandir = (tok, lits) =>
  tok.replace(/\u0001(\d+)\u0001/g, (m, i) => (Number(i) < lits.length ? lits[Number(i)] : m))

// CAMBIO 2.2: expandir ANTES de base(). Sin esto el token de COMANDO podía venir
// entrecomillado o con escape ('rm', \rm, "/bin/rm") y base() miraba un placeholder.
// TERCERA RONDA: el cierre del subshell viaja PEGADO al último token — `(rm -rf .)`
// dejaba el objetivo como ".)" y radio() no lo reconocía. Se saca solo el paréntesis que
// NO abrió este token, así $(pwd) y $(mktemp -d) quedan intactos.
function limpiar(tok, lits) {
  let t = expandir(tok, lits).replace(/^\\+/, "").replace(/^["']+|["']+$/g, "")
  while (
    t.endsWith(")") &&
    (t.match(/\(/g) || []).length < (t.match(/\)/g) || []).length
  )
    t = t.slice(0, -1)
  return t
}

// DOMAINSERV-114: heredoc y entrecomillado son DATOS, no comandos. No se borran: se
// reemplazan por un placeholder INDEXADO, así el comando que sí ejecuta su literal
// (sh -c, psql -c, ssh) lo recupera y mira adentro.
function enmascarar(texto, lits) {
  const push = (cuerpo) => {
    lits.push(cuerpo)
    return PH_A + (lits.length - 1) + PH_A
  }
  // CAMBIO 2.3: el terminador puede venir INDENTADO (<<-SQL con tabs). Con ^\1$ el
  // heredoc quedaba sin cerrar: evasión y a la vez falso positivo al documentarlo.
  const conHeredocs = texto.replace(
    /<<[-~]?\s*['"]?(\w+)['"]?([\s\S]*?)^[ \t]*\1[ \t]*$/gm,
    (m, marca, cuerpo) => push(cuerpo),
  )
  return enmascararComillas(conHeredocs, lits)
}

// TERCERA RONDA. Antes eran DOS regex independientes (todas las comillas simples, después
// todas las dobles) y eso rompía por los dos lados:
//   - `echo "it's" && rm -rf . && echo "that's"`: los dos apóstrofos de adentro pareaban
//     entre sí y se comían el rm del medio. El comando pasaba entero.
//   - `sh -c "psql -c \"DROP TABLE x\""`: el escape rompía el pareo y el literal interno
//     quedaba invisible (era el hueco 6 del header).
// Un escáner IZQUIERDA-A-DERECHA respeta lo que bash respeta: la comilla que abre PRIMERO
// manda, y adentro de "" el backslash escapa " \ $ ` y el newline.
function enmascararComillas(texto, lits) {
  const push = (cuerpo) => {
    lits.push(cuerpo)
    return PH_A + (lits.length - 1) + PH_A
  }
  const out = []
  let i = 0
  const n = texto.length
  while (i < n) {
    const c = texto[i]
    if (c === "\\" && i + 1 < n) {
      out.push(texto.slice(i, i + 2))
      i += 2
      continue
    }
    if (c === "'") {
      const j = texto.indexOf("'", i + 1)
      if (j < 0) {
        // comilla sin cerrar: no es un literal, es texto
        out.push(c)
        i++
        continue
      }
      out.push(push(texto.slice(i + 1, j)))
      i = j + 1
      continue
    }
    if (c === '"') {
      let j = i + 1
      let cerrado = false
      const buf = []
      while (j < n) {
        if (texto[j] === "\\" && j + 1 < n) {
          // dentro de "" el backslash SOLO escapa estos; con el resto se queda
          buf.push('"\\$`\n'.includes(texto[j + 1]) ? texto[j + 1] : texto.slice(j, j + 2))
          j += 2
          continue
        }
        if (texto[j] === '"') {
          cerrado = true
          break
        }
        buf.push(texto[j])
        j++
      }
      if (!cerrado) {
        out.push(c)
        i++
        continue
      }
      out.push(push(buf.join("")))
      i = j + 1
      continue
    }
    out.push(c)
    i++
  }
  return out.join("")
}

// Un # que abre palabra comenta hasta el fin de línea: NADA de ahí se ejecuta. Se strippea
// DESPUÉS del enmascarado, así un # dentro de comillas ya es un placeholder. Esto es lo que
// convertía `rm -rf $PWD # PWD=tmp` en una asignación falsa que sobrescribía $PWD, y de
// paso evita el falso positivo simétrico (`ls # rm -rf /`).
const sinComentarios = (texto) => texto.replace(/(^|[\s;&|(])#[^\n]*/gm, "$1")

const sinContinuaciones = (texto) => texto.replace(/\\\n/g, " ")

// Consume EN LOOP asignaciones (FOO=1), gramática de shell (then/do/{) y wrappers con el
// VALOR de sus flags. Antes era un regex de una pasada que consumía la flag pero no su
// valor: `sudo -u www-data rm -rf` pasaba.
function pelar(toks, lits) {
  let i = 0
  let cambio = true
  while (cambio && i < toks.length) {
    cambio = false
    while (i < toks.length && (/^\w+=/.test(toks[i]) || DESCARTES.has(toks[i]))) {
      i++
      cambio = true
    }
    if (i < toks.length && WRAPPERS.has(base(limpiar(toks[i], lits)))) {
      const w = base(limpiar(toks[i], lits))
      i++
      cambio = true
      while (i < toks.length) {
        const t = toks[i]
        if (t === "--") {
          i++
          break
        }
        if (t.startsWith("-") && t.length > 1) {
          i++
          if (!t.includes("=") && WRAP_VALOR.has(t) && i < toks.length) i++
          continue
        }
        if (w === "timeout" && /^\d+(?:\.\d+)?[smhd]?$/.test(t)) {
          i++
          continue
        }
        if (w === "env" && /^\w+=/.test(t)) {
          i++
          continue
        }
        break
      }
    }
  }
  return toks.slice(i)
}

// Los segmentos CRUDOS, en orden. Es el mismo corte que consume segmentos() y el que
// consume el mapa de asignaciones: el índice tiene que significar lo mismo en los dos,
// porque una asignación solo vale para los segmentos POSTERIORES.
function dividir(texto) {
  // TERCERA RONDA: `(rm -rf .)` no disparaba porque el token era "(rm". El paréntesis que
  // abre en POSICIÓN DE COMANDO se separa; el de $( no, porque ahí el $ lo precede y esta
  // clase no lo incluye (si se separara, $(pwd) y $(mktemp -d) se romperían).
  const t = sinContinuaciones(texto).replace(/(^|[\s;&|])\(/gm, "$1( ")
  return t.split(/\|\||&&|[;|\n&]/)
}

// Devuelve pares [idx, toks]: el idx es el que indexa el mapa de asignaciones.
function segmentos(texto, lits) {
  const out = []
  dividir(texto).forEach((seg, i) => {
    const toks = pelar(seg.split(/\s+/).filter(Boolean), lits)
    if (toks.length) out.push([i, toks])
  })
  return out
}

// Devuelve [cuerpo, envuelto, remoto]. envuelto: no se exige que rm sea el primer token
// (el incidente llegó como `docker exec <ctr> rm -f .env.qa`). remoto: no hay forma de
// resolver el radio del otro lado.
function desenvolver(toks, lits) {
  const c0 = base(limpiar(toks[0], lits))
  const subs = REMOTAS_SUB[c0]
  if (subs) {
    for (let i = 1; i < toks.length; i++) {
      if (subs.includes(toks[i])) return [pelar(toks.slice(i + 1), lits), true, true]
    }
    return [toks, false, false]
  }
  if (REMOTAS.has(c0)) return [pelar(toks.slice(1), lits), true, true]
  if (LOCALES.has(c0)) return [pelar(toks.slice(1), lits), true, false]
  return [toks, false, false]
}

// el NOMBRE del comando también puede venir de una variable (RM=rm; $RM -rf .), y comparar
// el token crudo contra "rm" lo dejaba pasar. Se resuelve igual que un objetivo.
function posicionesRm(toks, envuelto, lits, ctx) {
  const esRm = (t) => {
    t = limpiar(t, lits)
    if (base(t) === "rm") return true
    return ctx != null && base(preNormalizar(t, ctx)) === "rm"
  }
  if (envuelto) return toks.map((t, i) => (esRm(t) ? i : -1)).filter((i) => i >= 0)
  return esRm(toks[0]) ? [0] : []
}

// El cwd puede cambiar DENTRO del mismo comando: `cd <padre> && rm -rf <proyecto>` es el
// proyecto entero escrito como nombre relativo. Se sigue el cd cuando el destino es literal;
// si no se puede resolver, devuelve null y los relativos escalan.
function cwdTrasCd(toks, lits, ctx, cwdActual) {
  if (!toks.length || base(limpiar(toks[0], lits)) !== "cd") return cwdActual
  const args = toks.slice(1).filter((t) => !limpiar(t, lits).startsWith("-"))
  if (!args.length) return ctx.home || cwdActual
  const d = preNormalizar(limpiar(args[0], lits), ctx)
  if (certeza(d) !== "literal") return null
  return norm(d.startsWith("/") ? d : (cwdActual || ".") + "/" + d)
}

// Dónde termina el valor de una asignación. Respeta $( ) para que `d=$(mktemp -d)` sea UN
// valor y no "d=$(mktemp" más un token suelto.
function finDeValor(s, i) {
  let prof = 0
  while (i < s.length) {
    const c = s[i]
    if (s.slice(i, i + 2) === "$(") {
      prof++
      i += 2
      continue
    }
    if (c === "(" && prof) prof++
    else if (c === ")" && prof) prof--
    else if (!prof && (/\s/.test(c) || ";&|".includes(c))) break
    i++
  }
  return i
}

// Las asignaciones REALES de un segmento, con la semántica de bash:
//
//   1. tienen que estar al PRINCIPIO del segmento (tras un export/declare opcional);
//   2. si después de ellas queda un COMANDO, son un prefijo de ENTORNO y NO cambian las
//      variables del shell: bash expande los argumentos ANTES de armar ese entorno, así
//      que `PWD=tmp rm -rf $PWD` borra el cwd REAL. Se descartan.
//
// El barrido viejo era un matchAll de (\w+)=(\S*) sobre el TEXTO ENTERO, sin distinguir
// una asignación de un comentario, de un argumento ni de un prefijo de entorno. Eso daba
// un mapa que APAGABA el guard: `rm -rf $PWD # PWD=tmp`, `echo PWD=tmp; rm -rf $PWD` y
// `PWD=tmp rm -rf $PWD` pasaban los tres.
function asignacionesDe(seg, lits) {
  const vals = {}
  let i = 0
  for (;;) {
    const resto = seg.slice(i)
    const m = /^\s*(?:(?:export|declare|local|readonly|typeset)\s+)*(\w+)=/.exec(resto)
    if (!m) break
    const desde = i + m[0].length
    const j = finDeValor(seg, desde)
    vals[m[1]] = limpiar(seg.slice(desde, j), lits)
    i = j
  }
  if (seg.slice(i).trim()) return {} // prefijo de entorno: no toca el shell
  // PROTEGIDAS: ninguna asignación puede redefinir con qué se mide el radio.
  const out = {}
  for (const k of Object.keys(vals)) if (vals[k] && !PROTEGIDAS.has(k)) out[k] = vals[k]
  return out
}

// Para cada segmento, el mapa VIGENTE justo antes de ejecutarlo (y al final, el estado
// definitivo). Con esto una asignación POSTERIOR ya no resuelve un objetivo anterior
// (`rm -rf $D; D=/tmp/x` queda indecidible, que es lo correcto: cuando el rm corre, D no
// vale nada).
function mapaPorSegmento(texto, lits, baseVars) {
  let acc = { ...(baseVars || {}) }
  const salida = []
  for (const seg of dividir(texto)) {
    salida.push({ ...acc })
    const nuevas = asignacionesDe(seg, lits)
    for (const k of Object.keys(nuevas)) {
      // si la misma var se asigna dos veces distinto, queda sin resolver y manda el
      // fail-closed
      acc[k] = k in acc && acc[k] !== nuevas[k] ? "" : nuevas[k]
    }
    const limpio = {}
    for (const k of Object.keys(acc)) if (acc[k]) limpio[k] = acc[k]
    acc = limpio
  }
  salida.push({ ...acc }) // el estado FINAL: lo usa el chequeo del automarker
  return salida
}

function preNormalizar(t, ctx) {
  t = t.trim().replace(/^["']+|["']+$/g, "")
  // TERCERA RONDA: el escape INTERIOR. En una palabra sin comillas bash resuelve \X a X
  // (verificado read-only: `echo .gi\t` imprime .git), y el guard lo comparaba como texto,
  // así que .gi\t y ./\.git no se reconocían como el .git. Va acá y no en limpiar() porque
  // limpiar() también procesa el token de COMANDO y el \; de find -exec.
  t = t.replace(/\\(.)/g, "$1")
  // las vars del propio comando PRIMERO: así una cadena H=$HOME; rm -rf $H termina
  // resolviendo a $HOME y de ahí al path real
  const vals = ctx.vars || {}
  if (Object.keys(vals).length)
    t = t.replace(/\$\{(\w+)\}|\$(\w+)/g, (m, a, b) => {
      const k = a || b
      return Object.prototype.hasOwnProperty.call(vals, k) ? vals[k] : m
    })
  t = t.replace(/\$\((?:pwd|PWD)\)|`pwd`/g, "$PWD")
  t = t.replace(/^\$\{PWD\}/, "$PWD")
  if (!ctx.remoto) {
    // ~+ es literalmente $PWD (verificado: `echo ~+` imprime el cwd). Le faltaba, y
    // `rm -rf ~+` pasaba como si fuera un nombre de archivo raro.
    t = t.replace(/^~\+(?=\/|$)/, ctx.cwd || ".")
    t = t.replace(/^\$PWD(?=\/|$)/, ctx.cwd || ".")
    if (ctx.home) {
      t = t.replace(/^~(?=\/|$)/, ctx.home)
      t = t.replace(/^\$\{?HOME\}?(?=\/|$)/, ctx.home)
    }
  }
  return t
}

// Qué tan resoluble es el objetivo. Es el eje de la TERCERA RONDA: el guard resolvía lo que
// podía y, cuando NO podía, concluía "no es catastrófico" — fail-OPEN. Ahora la clase
// decide, y solo "literal" habilita comparar paths como texto.
//
//   opaco   el valor no se conoce: $VAR sin resolver, $(...), backticks, ~- ($OLDPWD),
//           ~usuario (el home de OTRO), ${VAR:-x} con operador.
//   glob    el valor lo produce BASH expandiendo metacaracteres (* ? [ ] { }).
//   literal el path es exactamente lo que dice.
function certeza(o) {
  if (o.includes("$") || o.includes("`") || o.startsWith("~")) return "opaco"
  return [...GLOB].some((c) => o.includes(c)) ? "glob" : "literal"
}

function flagsObjetivos(args, lits, ctx) {
  let rec = false
  let incierto = false
  const objetivos = []
  for (const a of args) {
    const t = limpiar(a, lits)
    if (["-", "--", "+", "{}", ";", ")", "\\;"].includes(t)) continue
    // TERCERA RONDA: preNormalizar va ANTES de clasificar. Con el orden invertido,
    // `F=-rf; rm $F .` caía en objetivos (no empieza con "-" TODAVÍA), rec quedaba en
    // false y la rama entera del radio se salteaba: era rm -rf del cwd, y pasaba.
    const p = preNormalizar(t, ctx)
    if (p.startsWith("--")) {
      if (p === "--recursive" || p === "--recursive=true") rec = true
      continue
    }
    if (p.startsWith("-") && p.length > 1 && !/[0-9]/.test(p[1])) {
      if (p.slice(1).includes("r") || p.slice(1).includes("R")) rec = true
      continue
    }
    objetivos.push(p)
    if (certeza(p) !== "literal") incierto = true
  }
  return [rec, objetivos, incierto]
}

function igualOAncestro(a, b) {
  if (!a || !b) return false
  a = a.replace(/\/+$/, "") || "/"
  b = b.replace(/\/+$/, "") || "/"
  return a === "/" || a === b || b.startsWith(a + "/")
}

function bajo(a, prefijos) {
  a = a.replace(/\/+$/, "") || "/"
  return prefijos.some((p) => a === p || a.startsWith(p + "/"))
}

function rutinario(a) {
  const b = base(a.replace(/\/+$/, ""))
  return RUTINA.has(b) || b.startsWith("coverage")
}

// $DIR/build no puede SER el proyecto ni un ancestro: hay al menos un componente
// concreto después de la expansión. $DIR solo, sí.
function sufijoConcreto(o) {
  const partes = o.split("/")
  let ult = -1
  partes.forEach((p, i) => {
    if (p.includes("$") || p.includes("`")) ult = i
  })
  const cola = partes.slice(ult + 1).filter(Boolean)
  if (!cola.length) return false
  // TERCERA RONDA: antes solo se exigía que la cola no fuera . ni .. ni llevara *. Con eso
  // ${DIR:-/} contaba como sufijo concreto (su cola era "}") y el fail-closed no corría;
  // lo mismo ~usuario. Ahora la cola tiene que ser un nombre LITERAL.
  return cola.every((p) => /^[\w.@+-]+$/.test(p) && p !== ".." && p !== ".")
}

function ancestros(p) {
  const out = []
  let a = (p || "").replace(/\/+$/, "")
  while (a && a !== "/") {
    out.push(a)
    a = a.includes("/") ? a.slice(0, a.lastIndexOf("/")) || "/" : ""
  }
  out.push("/")
  return out
}

// El conjunto EXACTO de paths cuyo borrado radio() considera catastrófico. Se usa para
// decidir si un GLOB puede expandir a alguno de ellos, en vez de compararlo como texto.
function candidatosRadio(ctx) {
  const c = ["/"]
  for (const p of [ctx.cwd, ctx.raiz, ctx.home]) {
    if (p) {
      c.push(...ancestros(p))
      c.push(norm(p + "/.git"))
    }
  }
  for (const r of RAICES_APP) c.push(...ancestros(r))
  return [...new Set(c)]
}

// glob de shell -> regex. * no cruza barras, ** sí, {a,b} es alternancia, [!x] niega.
function globCuerpo(g) {
  const out = []
  let i = 0
  while (i < g.length) {
    const c = g[i]
    if (c === "*") {
      const doble = g.slice(i, i + 2) === "**"
      out.push(doble ? ".*" : "[^/]*")
      i += doble ? 2 : 1
      continue
    }
    if (c === "?") {
      out.push("[^/]")
      i++
      continue
    }
    if (c === "[") {
      const j = g.indexOf("]", i + 2)
      if (j > 0) {
        let cuerpo = g.slice(i + 1, j)
        if (cuerpo[0] === "!" || cuerpo[0] === "^") cuerpo = "^" + cuerpo.slice(1)
        out.push("[" + cuerpo + "]")
        i = j + 1
        continue
      }
    }
    if (c === "{") {
      const j = g.indexOf("}", i + 1)
      if (j > 0) {
        out.push(
          "(?:" +
            g
              .slice(i + 1, j)
              .split(",")
              .map(globCuerpo)
              .join("|") +
            ")",
        )
        i = j + 1
        continue
      }
    }
    out.push(c.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
    i++
  }
  return out.join("")
}

function alcanzaRadio(o, ctx) {
  const a = norm(o.startsWith("/") ? o : (ctx.cwd || ".") + "/" + o)
  let rx
  try {
    rx = new RegExp("^" + globCuerpo(a) + "$")
  } catch {
    return true // un patrón que no se puede modelar no se declara benigno
  }
  return candidatosRadio(ctx).some((c) => rx.test(c))
}

// TERCERA RONDA. El guard solo trataba el glob trailing /*, así que `.g*` (= .git .github
// .gitignore), `.*`, `.gi?`, `{.git,dist}` y `/ap*` le parecían nombres literales y
// pasaban. Y el ruido va del otro lado: node_modules/* o coverage* NO pueden escalar. Las
// dos reglas juntas dan las dos cosas:
//   1. D/* es "vaciar D", que equivale a borrar D (así ya disparaban /*, ./* y ../*);
//   2. el patrón se convierte a regex y se prueba contra candidatosRadio(): si PUEDE
//      expandir al cwd, al proyecto, a un ancestro o al .git, escala; si su prefijo literal
//      ya lo encierra en un subdirectorio acotado, no matchea nada y pasa.
function radioGlob(o, ctx) {
  let d = o.replace(/\/\*+$/, "/")
  if (d === "*" || d === "*/") d = "."
  if (certeza(d) === "literal" && radio(d, ctx)) return "radio"
  return alcanzaRadio(o, ctx) ? "radio" : ""
}

// ¿el objetivo de un rm RECURSIVO tiene radio catastrófico?
function radio(o, ctx) {
  o = (o || "").trim()
  if (!o) return false
  let g = o.replace(/\/\*+$/, "/")
  if (g === "*" || g === "*/") g = "."
  // el .git ENTERO (no un archivo de adentro: rm -rf .git/index.lock es rutina) es el
  // radio máximo posible — se lleva puesto justo lo que hace recuperable a todo lo demás
  if (base(g.replace(/\/+$/, "")) === ".git") return true
  if (ctx.remoto) {
    if ([".", "./", "$PWD", "$PWD/", "${PWD}"].includes(g)) return true
    if (["~", "~/", "$HOME", "$HOME/", "${HOME}", "${HOME}/"].includes(g)) return true
    if (g === ".." || g === "../" || g.startsWith("../")) return true
    if (!g.startsWith("/")) return false
    const a = norm(g)
    if (a === "/") return true
    if (/^\/home(?:\/[^/]+)?\/?$/.test(a)) return true
    if (rutinario(a) || bajo(a, EFIMEROS)) return false
    // container/host remoto: cualquier absoluto puede ser un bind mount del repo
    return true
  }
  if (["$PWD", "$PWD/", "${PWD}", "$HOME", "$HOME/", "${HOME}", "~", "~/"].includes(g)) return true
  const a = norm(g.startsWith("/") ? g : (ctx.cwd || ".") + "/" + g)
  if (a === "/") return true
  if (igualOAncestro(a, ctx.cwd)) return true
  if (ctx.raiz && igualOAncestro(a, ctx.raiz)) return true
  if (ctx.home && igualOAncestro(a, ctx.home)) return true
  if (/^\/home(?:\/[^/]+)?\/?$/.test(a)) return true
  if (RAICES_APP.some((r) => igualOAncestro(a, r))) return true
  // TERCERA RONDA: /srv/<app> y /var/www/<sitio> SON la app desplegada, no un
  // subdirectorio cualquiera de ella — el evasor medido era `sudo -u www-data rm -rf
  // /srv/domain`, que es exactamente el deploy de este repo. Un hijo de RUTINA (/app/dist,
  // /app/node_modules) sigue siendo rutina y no llega acá.
  const limpio = a.replace(/\/+$/, "") || "/"
  const padre = limpio.includes("/") ? limpio.slice(0, limpio.lastIndexOf("/")) || "/" : ""
  return RAICES_APP.includes(padre) && !rutinario(a)
}

function trackeado(a, ctx) {
  const d = a.includes("/") ? a.slice(0, a.lastIndexOf("/")) : ctx.cwd || "."
  try {
    execFileSync("git", ["-C", d || "/", "ls-files", "--error-unmatch", "--", base(a)], {
      stdio: "ignore",
      timeout: 3000,
    })
    return true
  } catch {
    return false
  }
}

function sensiblePath(o, ctx) {
  const b = base(o).toLowerCase()
  if (!b || b === "." || b === "..") return false
  if (EJEMPLAR.test(b)) return false
  if (!SENSIBLES.some((re) => re.test(b))) return false
  if (!ctx.remoto) {
    const a = norm(o.startsWith("/") ? o : (ctx.cwd || ".") + "/" + o)
    // un secreto suelto en /tmp es una COPIA de scratch, no el original del repo (falso
    // positivo medido: rm /tmp/no-secret.txt). Se exige que NO cuelgue del cwd ni del
    // HOME, porque un cwd bajo /tmp sigue siendo el proyecto parado.
    if (bajo(a, EFIMEROS) && !bajo(a, [ctx.cwd, ctx.home].filter(Boolean))) return false
    // trackeado en git: `git checkout --` lo recupera y manda el git-guard
    if (trackeado(a, ctx)) return false
  }
  return true
}

// EL PRINCIPIO (tercera ronda): si el objetivo de un rm RECURSIVO no es un literal que se
// pueda resolver con CERTEZA, se ESCALA. Antes el guard intentaba resolver y, cuando no
// podía, concluía "no es catastrófico" — fail-OPEN, y era la causa única de los tres bugs
// que el juez encontró.
function rmPeligroso(args, lits, ctx) {
  const [rec, objetivos, incierto] = flagsObjetivos(args, lits, ctx)
  // un rm recursivo SIN objetivo explícito lo recibe por stdin (echo . | xargs rm -rf):
  // el objetivo no está en el texto, así que no hay nada que resolver y se escala.
  // Excepción: en `find ... -exec rm -rf {} +` el objetivo lo pone find y es findPeligroso
  // quien mira sus filtros — sin esto, borrar node_modules con find pasaba a ser ruido.
  if (rec && objetivos.length === 0 && !args.some((a) => limpiar(a, lits).includes("{}")))
    return "radio-indecidible"
  if (rec || incierto) {
    for (const o of objetivos) {
      const clase = certeza(o)
      if (clase === "opaco") {
        // del otro lado del ssh/docker no hay cwd, pero $PWD/$HOME/~ siguen significando
        // algo catastrófico: radio() ya los cubre.
        if (ctx.remoto && radio(o, ctx)) return "radio"
        // un valor derivado de mktemp es un directorio NUEVO por construcción: no puede
        // ser el proyecto ni un ancestro
        if (o.includes("mktemp")) continue
        // fail-closed: no se puede resolver el objetivo de un rm recursivo
        if (rec && !sufijoConcreto(o)) return "radio-indecidible"
        continue
      }
      if (clase === "glob") {
        // un glob NO recursivo borra archivos de un directorio, no el árbol: `rm -f *.log`
        // es rutina y no puede costar un disparo.
        if (!rec) continue
        const motivo = ctx.remoto ? (radio(o, ctx) ? "radio" : "") : radioGlob(o, ctx)
        if (motivo) return motivo
        continue
      }
      if (radio(o, ctx)) return "radio"
    }
  }
  for (const o of objetivos) {
    if (certeza(o) !== "literal") continue
    if (sensiblePath(o, ctx)) return "sensible"
  }
  return ""
}

function findPeligroso(toks, lits, ctx) {
  if (base(limpiar(toks[0], lits)) !== "find") return ""
  const resto = toks.slice(1)
  const raices = []
  for (const t of resto) {
    if (t.startsWith("-") || ["(", "!", ")"].includes(t)) break
    raices.push(t)
  }
  let borra = false
  let filtrado = false
  resto.forEach((t, i) => {
    if (t === "-delete") borra = true
    if (
      ["-exec", "-execdir", "-ok", "-okdir"].includes(t) &&
      resto
        .slice(i + 1, i + 3)
        .some((x) => ["rm", "shred", "unlink"].includes(base(limpiar(x, lits))))
    )
      borra = true
    if (["-name", "-iname", "-path", "-ipath", "-regex", "-wholename"].includes(t)) {
      const pat = i + 1 < resto.length ? limpiar(resto[i + 1], lits) : ""
      if (pat !== "*" && pat !== ".*" && pat !== "") filtrado = true
    }
  })
  if (!borra || filtrado) return ""
  for (const r of raices) {
    if (radio(preNormalizar(limpiar(r, lits), ctx), ctx)) return "radio"
  }
  return ""
}

function sqlDestructivo(sql) {
  // CAMBIO 2.8: /*…*/ primero — DROP/**/TABLE evadía y un /* WHERE */ comentado apagaba
  // la regla.
  const limpio = sql.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/--[^\n]*/g, " ")
  for (const st of limpio.split(";")) {
    if (
      /\bDROP\s+(?:DATABASE|TABLE|SCHEMA|OWNED\s+BY|ROLE|USER|INDEX|VIEW|MATERIALIZED\s+VIEW|TYPE|EXTENSION|SEQUENCE|FUNCTION|TRIGGER|TABLESPACE)\b/i.test(st)
    )
      return true
    if (/\bALTER\s+TABLE\b[\s\S]*?\bDROP\s+(?:COLUMN|CONSTRAINT)\b/i.test(st)) return true
    if (/\bTRUNCATE\b/i.test(st)) return true
    // WHERE true / WHERE 1=1 no filtra NADA: es un DELETE completo con disfraz
    const sw = st.replace(/\bWHERE\s+(?:true|1\s*=\s*1)\b/gi, " ")
    if (/\bWHERE\b/i.test(sw)) continue
    if (/\bDELETE\s+FROM\b/i.test(sw)) return true
    if (/\bUPDATE\b[\s\S]*?\bSET\b/i.test(sw)) return true
  }
  return false
}

function literalesDe(toks, lits) {
  const out = []
  for (const m of toks.join(" ").matchAll(/\u0001(\d+)\u0001/g)) {
    const i = Number(m[1])
    if (i < lits.length) out.push(lits[i])
  }
  return out
}

// El cliente tiene que estar en POSICIÓN DE COMANDO (mismo criterio que DOMAINSERV-146
// para rm). Con any(token) alcanzaba que la palabra apareciera: `docker ps | grep -i
// mysql` y `which mysql` disparaban sql-opaco.
function tieneCliente(parte, lits) {
  const toks = pelar(parte.split(/\s+/).filter(Boolean), lits)
  if (!toks.length) return false
  const [cuerpo, envuelto] = desenvolver(toks, lits)
  if (!cuerpo.length) return false
  if (SQL_CLIENTES.has(base(limpiar(cuerpo[0], lits)))) return true
  return envuelto && cuerpo.some((t) => SQL_CLIENTES.has(base(limpiar(t, lits))))
}

// El archivo que el cliente SQL va a ejecutar: si se puede LEER, se analiza (mucho mejor
// que escalar a ciegas); si no, se escala. `-f -` es stdin y lo cubre el análisis del
// pipeline. Con ctx remoto el path es del OTRO lado: no se lee.
function sqlDeArchivo(path, ctx) {
  if (path === "-" || path === "/dev/stdin") return ""
  if (ctx.remoto) return "sql-opaco"
  const p = preNormalizar(path, ctx)
  if (p.includes("$") || p.includes("`") || p.includes("*")) return "sql-opaco"
  const a = p.startsWith("/") ? p : (ctx.cwd || ".") + "/" + p
  let texto
  try {
    if (statSync(a).size > 262144) return "sql-opaco"
    texto = readFileSync(a, "utf8")
  } catch {
    return "sql-opaco"
  }
  return sqlDestructivo(texto) ? "sql" : ""
}

function archivosSql(tramo, toks, lits) {
  const paths = []
  for (const m of tramo.matchAll(/(?<!<)<(?!<)\s*([^\s;|&<>]+)/g)) paths.push(limpiar(m[1], lits))
  if (toks.some((t) => base(t) === "psql")) {
    toks.forEach((t, i) => {
      if ((t === "-f" || t === "--file") && i + 1 < toks.length) paths.push(limpiar(toks[i + 1], lits))
      else if (t.startsWith("--file=")) paths.push(limpiar(t.slice("--file=".length), lits))
    })
  }
  return paths
}

// CAMBIO 2.8: el PIPELINE completo. Antes se miraba segmento por segmento y el pipe los
// separaba: `echo "DROP TABLE x" | psql` pasaba.
function sqlEnPipeline(texto, lits, ctx) {
  for (const tramo of sinContinuaciones(texto).split(/&&|\|\||[;\n]/)) {
    const partes = tramo.split("|")
    const con = partes.map((p, i) => (tieneCliente(p, lits) ? i : -1)).filter((i) => i >= 0)
    if (!con.length) continue
    const toks = tramo.split(/\s+/).filter(Boolean)
    const adentro = literalesDe(toks, lits)
    if (adentro.some(sqlDestructivo)) return "sql"
    // el archivo que ejecuta el cliente: se LEE si se puede, y si no se escala
    for (const p of archivosSql(tramo, toks, lits)) {
      const m = sqlDeArchivo(p, ctx)
      if (m) return m
    }
    if (adentro.some((s) => /\\ir?\s/.test(s))) return "sql-opaco"
    if (con[0] > 0) {
      const arriba = partes[0].split(/\s+/).filter(Boolean)
      if (arriba.length && !["echo", "printf"].includes(base(limpiar(arriba[0], lits))))
        return "sql-opaco"
    }
  }
  return ""
}

// CAMBIO 3: el agente podía escribirse el marker de bypass con un `echo >`. Autorizar un
// borrado tiene que costar una intervención HUMANA, no una llamada de Bash del agente.
function escribeAutomarker(texto, lits, ctx) {
  // TERCERA RONDA: el path puede venir ENTERO en una variable, y ahí el literal
  // "destructive-bypass" no está en la línea — `M=$HOME/...-bypass-x; echo r > $M`.
  const apunta = (t) => {
    const e = expandir(t, lits)
    return e.includes("destructive-bypass") || preNormalizar(e, ctx).includes("destructive-bypass")
  }
  for (const m of texto.matchAll(/>>?\s*([^\s;|&<>]+)/g)) {
    if (apunta(m[1])) return true
  }
  for (const [, toks] of segmentos(texto, lits)) {
    const c0 = base(limpiar(toks[0], lits))
    // un intérprete escribe desde su propio literal, sin pasar por una redirección:
    // python3 -c "open(...,'w')". grep/ls NO están en estas listas, así que inspeccionar el
    // marker sigue siendo gratis.
    if (VERBOS_ESCRITURA.has(c0) || INTERPRETES_ESCRITURA.has(c0)) {
      if (toks.slice(1).some(apunta)) return true
    }
  }
  return false
}

function hayInterprete(toks, lits) {
  for (let i = 0; i < toks.length; i++) {
    const b = base(limpiar(toks[i], lits))
    if (b === "eval") return true
    if (SHELLS.has(b) && toks.slice(i + 1, i + 4).some((x) => /^-\w*c$/.test(limpiar(x, lits))))
      return true
  }
  return false
}

export function isDestructiveCommand(texto, ctx, lits, hondura) {
  lits = lits || []
  hondura = hondura || 0
  ctx = ctx || ctxBase(null)
  texto = sinComentarios(enmascarar(String(texto || ""), lits))
  // El mapa de asignaciones es POSICIONAL: mapas[i] es lo que vale justo antes del segmento
  // i, y mapas[mapas.length-1] el estado final. Sin esto una asignación POSTERIOR resolvía
  // un objetivo anterior (`rm -rf $D; D=/tmp/x` pasaba como si D valiera /tmp/x).
  const mapas = mapaPorSegmento(texto, lits, ctx.vars)
  const final = { ...ctx, vars: mapas[mapas.length - 1] }
  if (escribeAutomarker(texto, lits, final)) return "automarker"
  let motivo = sqlEnPipeline(texto, lits, final)
  if (motivo) return motivo
  let cwdVivo = ctx.cwd
  for (const [idx, toks] of segmentos(texto, lits)) {
    const cur = { ...ctx, cwd: cwdVivo, vars: idx < mapas.length ? mapas[idx] : mapas[mapas.length - 1] }
    cwdVivo = cwdTrasCd(toks, lits, cur, cwdVivo)
    const [cuerpo, envuelto, remoto] = desenvolver(toks, lits)
    if (!cuerpo.length) continue
    const sub = { ...cur, remoto: ctx.remoto || remoto }
    for (const i of posicionesRm(cuerpo, envuelto, lits, sub)) {
      motivo = rmPeligroso(cuerpo.slice(i + 1), lits, sub)
      if (motivo) return motivo
    }
    motivo = findPeligroso(toks, lits, cur)
    if (motivo) return motivo
    const adentro = literalesDe(cuerpo, lits)
    // CAMBIO 2.1: recursar también en el literal de las ENVOLTURAS. El deploy de este
    // repo tiene la forma `ssh vps "rm -rf /srv/domain"` y pasaba porque solo se
    // recursaba con sh -c/eval.
    if (hondura < 3 && adentro.length && (envuelto || hayInterprete(cuerpo, lits))) {
      for (const s of adentro) {
        motivo = isDestructiveCommand(s, sub, lits, hondura + 1)
        if (motivo) return motivo
      }
    }
  }
  return ""
}

const DETALLE = {
  radio:
    "borrado RECURSIVO cuyo objetivo es el directorio donde estás parado, la raíz del repo, un ANCESTRO de cualquiera de los dos, /, $HOME, el .git, o una raíz de app de container (/app /srv /repo /workspace /var/www /usr/src/app — el incidente fue justo eso: /app era un bind mount del repo del host y no lo parecía). Un rm -rf de node_modules, dist, build, vendor o cualquier subdirectorio que NO sea el proyecto entero NO llega acá.",
  "radio-indecidible":
    "borrado RECURSIVO cuyo objetivo no se puede resolver (lleva $VAR, $(…) o backticks sin sufijo concreto). El guard NO asume que es inofensivo: si la variable vale / o $HOME el daño es total, así que escala.",
  sensible:
    "rm de un archivo SENSIBLE que NO está trackeado en git (.env*, *.key, *.pem, id_rsa, *credential*, *secret*, *.p12) — es el incidente original, `docker exec <ctr> rm -f .env.qa`. Un archivo trackeado no llega acá: ese lo recupera git checkout -- y lo cubre el git-guard.",
  sql:
    "SQL destructivo (DROP DATABASE/TABLE/SCHEMA/ROLE/INDEX/VIEW/TYPE/EXTENSION/OWNED BY, ALTER TABLE … DROP COLUMN, TRUNCATE, DELETE/UPDATE sin WHERE real — WHERE true y WHERE 1=1 cuentan como SIN where).",
  "sql-opaco":
    "un cliente SQL que ejecuta un archivo o un stdin que el guard NO puede leer (-f, <, \\i, un pipe que no es echo). No se puede afirmar que sea benigno, así que escala.",
  automarker:
    "el comando ESCRIBE el marker de bypass de este guard. Autorizar un borrado irreversible tiene que costar una intervención HUMANA fuera del agente, no una llamada de Bash del agente. Ningún bypass habilita esta operación: pedíselo al humano.",
}

// OpenCode no tiene el par ask/deny de Claude Code: un plugin solo puede throw. Por eso
// acá SIEMPRE es deny duro (equivalente a bypassPermissions) y la única vía de autorizar
// un borrado legítimo es el marker de un solo uso, que escribe el HUMANO.
function guardDestructivo(cmd, sessionID, directory) {
  const motivo = isDestructiveCommand(cmd, ctxBase(directory))
  if (!motivo) return
  const bypass = join(STATE_DIR, "destructive-bypass-" + sessionID)
  // El bypass NO habilita escribirse el propio bypass: si lo hiciera, un agente que ya
  // consumió uno podría renovárselo solo. automarker es siempre humano-only.
  if (motivo !== "automarker") {
    try {
      readFileSync(bypass, "utf8")
      try {
        unlinkSync(bypass) // un solo uso: no habilita la sesión entera
      } catch {}
      return
    } catch {}
  }
  throw new Error(
    "domain destructive-guard (DOMAINSERV-222) [" +
      motivo +
      "]: " +
      DETALLE[motivo] +
      (motivo === "automarker"
        ? ""
        : " Nada de esto se recupera con git. Si el borrado es legítimo, el HUMANO autoriza UNO SOLO con: echo 'tu razón' > " +
          bypass),
  )
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

      // GUARD DESTRUCTIVO (DOMAINSERV-222): va PRIMERO, antes incluso del commit-gate.
      // Un borrado irreversible no puede esperar a que se decida si el comando edita
      // código: cuando el rm corre, ya no hay nada que gatear.
      if (tool === "bash") guardDestructivo(cmd, sessionID, directory)

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
