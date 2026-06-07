package commands

import (
	"fmt"
	"os"
)

// completion imprime el script de autocompletado para bash/zsh/fish.
func completion(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain completion bash|zsh|fish")
		return 2
	}
	switch args[0] {
	case "bash":
		fmt.Print(bashCompletion)
		return 0
	case "zsh":
		fmt.Print(zshCompletion)
		return 0
	case "fish":
		fmt.Print(fishCompletion)
		return 0
	}
	fmt.Fprintln(os.Stderr, "shell no soportado")
	return 2
}

const bashCompletion = `# bash completion for domain CLI
_domain_complete() {
  local cur prev resources
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  resources="projects observations obs agents flows skills search context completion help"

  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=( $(compgen -W "$resources" -- "$cur") )
    return 0
  fi

  case "$prev" in
    projects|agents|flows)
      COMPREPLY=( $(compgen -W "ls get run create" -- "$cur") )
      ;;
    observations|obs)
      COMPREPLY=( $(compgen -W "ls save" -- "$cur") )
      ;;
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
      ;;
  esac
}
complete -F _domain_complete domain
`

const zshCompletion = `#compdef domain
_domain() {
  local -a resources
  resources=(
    'projects:Manage projects'
    'observations:Manage observations (alias: obs)'
    'agents:Manage agents'
    'flows:Manage flows'
    'skills:List skills'
    'search:Global search'
    'context:Show context snapshot'
    'completion:Generate shell completion'
  )
  _describe 'commands' resources
}
compdef _domain domain
`

const fishCompletion = `# fish completion for domain CLI
complete -c domain -n "__fish_use_subcommand" -a "projects" -d "Manage projects"
complete -c domain -n "__fish_use_subcommand" -a "observations obs" -d "Manage observations"
complete -c domain -n "__fish_use_subcommand" -a "agents" -d "Manage agents"
complete -c domain -n "__fish_use_subcommand" -a "flows" -d "Manage flows"
complete -c domain -n "__fish_use_subcommand" -a "skills" -d "List skills"
complete -c domain -n "__fish_use_subcommand" -a "search" -d "Global search"
complete -c domain -n "__fish_use_subcommand" -a "context" -d "Context snapshot"
complete -c domain -n "__fish_use_subcommand" -a "completion" -d "Shell completion"
`
