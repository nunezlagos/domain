// domain-git-guard.js — DOMAINSERV-69b
// Guard determinista de git destructivo para OpenCode, a nivel plugin
// (tool.execute.before). Espeja el git-guard de Claude Code
// (domain-pre-edit.sh, DOMAINSERV-73): normaliza el argv de git quitando las
// opciones globales (-C / -c / --git-dir / --work-tree) ANTES de matchear, para
// cerrar la evasión `git -C . reset --hard` que las reglas declarativas de
// opencode.json (permission.bash) NO pueden detectar por ser prefix-match.
//
// Alcance: SOLO git destructivo. NO cubre el gate SDD (flow activo) ni el
// commit-gate — eso requiere el ciclo de marker server-side + session_id y vive
// en un ticket aparte.

// DOMAINSERV-247: `clean` y `stash` matcheaban el subcomando entero, así que bloqueaban también
// las variantes de LECTURA — medido: git stash list, git stash show, git clean -n y
// git clean --dry-run. Un guard que impide mirar el estado obliga a trabajar a ciegas y empuja a
// desactivarlo entero, que es el modo de falla de DOMAINSERV-111/175/195.
//
// Ahora se nombra lo que MUTA en vez de excluir lo que lee: una lista de subcomandos destructivos
// es cerrada y conocida, mientras que las variantes de lectura crecen con cada versión de git.
// Enumerar lo peligroso deja lo nuevo por defecto permitido; enumerar lo seguro lo dejaría
// bloqueado, que es peor para un guard que ya demostró fallar por exceso.
const DESTRUCTIVE = [
  /git\s+reset\s+--hard/,
  // clean sin -n/--dry-run: cualquier otra combinación de flags borra archivos
  /git\s+clean\s+(?!.*(?:-n\b|--dry-run\b))\S/,
  // de stash solo mutan pop, drop, apply, clear y el push/save implícito
  /git\s+stash\s*(?:$|\s+(?:pop|drop|apply|clear|push|save|store|create|branch)\b)/,
  /git\s+checkout\s+(--|\.)/,
  /git\s+restore\b/,
  /git\s+rm\b/,
  /git\s+worktree\s+remove\b/,
]

// isDestructiveGit normaliza el comando y devuelve true si contiene un git
// mutante destructivo en cualquier posición del token stream.
export function isDestructiveGit(cmd) {
  const normalized = String(cmd || "").replace(
    /\bgit\s+(?:-[cC]\s+\S+\s+|--(?:git-dir|work-tree)(?:=\S+|\s+\S+)?\s+)*/g,
    "git ",
  )
  return DESTRUCTIVE.some((re) => re.test(normalized))
}

export const DomainGitGuard = async () => {
  return {
    "tool.execute.before": async (input, output) => {
      if (!input || input.tool !== "bash") return
      const cmd = output && output.args ? output.args.command : ""
      if (isDestructiveGit(cmd)) {
        throw new Error(
          "domain git-guard: comando git destructivo bloqueado " +
            "(reset --hard / clean / stash / checkout -- | . / restore / rm / worktree remove). " +
            "El agente NUNCA ejecuta git mutante sobre tu working tree. " +
            "Si de verdad lo necesitas, córrelo vos manualmente fuera del agente.",
        )
      }
    },
  }
}
