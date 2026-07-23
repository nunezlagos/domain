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

const DESTRUCTIVE = [
  /git\s+reset\s+--hard/,
  /git\s+clean\b/,
  /git\s+stash\b/,
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
