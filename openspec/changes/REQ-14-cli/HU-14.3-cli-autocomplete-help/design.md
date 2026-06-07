# Design: HU-14.3-cli-autocomplete-help

## Decisión arquitectónica

**Cobra nativo para completion:** Cobra ya incluye `GenBashCompletion`, `GenZshCompletion`, `GenFishCompletion`. Solo necesitamos exponerlos como subcomandos. No requiere librerías externas.

**Custom help template:** Override del template por defecto de Cobra para incluir ejemplos consistentes en todos los comandos. Cada comando define su `Example` field.

**Error suggestion:** Hook `SuggestFor` en Cobra + Levenshtein distance personalizado para flags (Cobra no sugiere flags por defecto).

**Help examples por comando:**
```go
var memorySaveCmd = &cobra.Command{
    Use:   "save",
    Short: "Save a new memory observation",
    Example: `  # Save a simple memory
  domain memory save --title "Fix login bug" --content "Fixed the login timeout issue" --type fix

  # Save a memory with project scope
  domain memory save --title "Architecture decision" --content "Using Postgres for storage" --type decision --project Domain

  # Save without content (title-only)
  domain memory save --title "Quick note"`,
}
```

**Man page generation:**
```go
var manCmd = &cobra.Command{
    Use:   "man",
    Short: "Generate man page",
    RunE: func(cmd *cobra.Command, args []string) error {
        header := &cobra.GenManHeader{
            Title:   "MEMORIA",
            Section: "1",
        }
        return rootCmd.GenMan(header, os.Stdout)
    },
}
```

## Alternativas descartadas

1. **Carapace para completion:** Librería externa potente pero añade dependencia. Cobra nativo es suficiente para nuestro caso de uso.
2. **No customizar help template:** El default de Cobra es genérico. Customizarlo mejora UX con ejemplos y formato consistente.
3. **No sugerir errores:** Cobra no lo hace por defecto. Implementar Levenshtein simple es fácil y mejora mucho la UX.

## Diagrama

```
Error path:
domain memry list
  → Cobra: unknown command "memry"
  → SuggestFor hook: levenshtein("memry", ["memory", "skill", ...]) → "memory"
  → Output: "Did you mean this?\n  memory"

Help path:
domain memory save --help
  → Cobra: print custom template
  → Template includes: Usage, Description, Examples, Flags, Global Flags

Completion path:
domain completion bash
  → Cobra: GenBashCompletion(os.Stdout)
  → Output: bash completion script
```

## TDD plan

1. **Red:** Test suggestions para typo conocido
2. **Green:** Implementar levenshtein + SuggestFor
3. **Refactor:** Extraer lev distance a util
4. **Iterar:** Custom help template, man page, completion
5. **Sabotaje:** Sacar ejemplo de un comando → test detecta

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Levenshtein da falsos positivos | Distancia máxima 3, solo sugerir si hay match cercano |
| Man page se desactualiza | Generar en CI, verificar con test que GenMan no da error |
| Completion scripts no funcionan en todos los shells | Testear en CI con bash, zsh, fish |
