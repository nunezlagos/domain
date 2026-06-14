# Proposal: issue-14.3-cli-autocomplete-help

## Intención

Implementar autocompletado para bash/zsh/fish, ayuda detallada con ejemplos en todos los comandos, detección de errores con sugerencias (did you mean?), y generación de man page.

## Scope

**Incluye:**
- `completion` subcomando con soporte bash/zsh/fish/powershell
- `--help` detallado con ejemplos en todos los commands y subcommands
- Error suggestions con Levenshtein distance para comandos y flags
- `man` subcomando que genera man page
- `--version` flag
- `--verbose` flag con debug info
- Template de ayuda customizado (override de Cobra default)

**Excluye:**
- Documentación externa (man y --help son suficientes)
- Tutorial interactivo

## Enfoque técnico

**Completion (Cobra nativo):**
```go
var completionCmd = &cobra.Command{
    Use:   "completion [bash|zsh|fish|powershell]",
    Short: "Generate shell completion script",
    Long:  `...`,
}

func init() {
    rootCmd.AddCommand(completionCmd)
    completionCmd.AddCommand(
        completionBashCmd,
        completionZshCmd,
        completionFishCmd,
        completionPowerShellCmd,
    )
}

var completionBashCmd = &cobra.Command{
    Use:   "bash",
    Short: "Generate bash completion",
    RunE: func(cmd *cobra.Command, args []string) error {
        return rootCmd.GenBashCompletion(os.Stdout)
    },
}
```

**Custom help template:**
```go
const customHelpTemplate = `{{.Short}}

Usage: {{.UseLine}}{{if .Aliases}}

Aliases: {{.Aliases}}{{end}}{{if .Example}}

Examples:
{{.Example}}{{end}}{{if .Commands}}

Available Commands:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .LocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .InheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}
`

func init() {
    rootCmd.SetHelpTemplate(customHelpTemplate)
}
```

**Error suggestions:**
```go
import "github.com/posener/complete/v2"

func suggestCommand(input string, valid []string) string {
    minDist := 3
    best := ""
    for _, v := range valid {
        dist := levenshteinDistance(input, v)
        if dist < minDist {
            minDist = dist
            best = v
        }
    }
    if best != "" {
        return fmt.Sprintf("Did you mean this?\n  %s", best)
    }
    return ""
}
```

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| Levenshtein en cada error puede ser lento con muchos comandos | Solo comparar contra comandos del mismo nivel, no todos |
| Man page se desincroniza con --help | Generar man page desde Cobra automáticamente |
| Completion scripts varían entre shells | Usar Cobra.GenXCompletion que ya maneja diferencias |

## Testing

- Unit: lev distance matching
- Integration: --help output matches golden file
- Integration: error suggestion for known typos
- Manual: source completion y probar en bash/zsh
