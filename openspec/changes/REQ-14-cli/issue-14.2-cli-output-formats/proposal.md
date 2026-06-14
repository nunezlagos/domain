# Proposal: issue-14.2-cli-output-formats

## Intención

Implementar un sistema de formatos de salida intercambiables (table, json, yaml) con detección automática de TTY vs pipe. Incluye colorización en modo interactivo y spinner para operaciones largas.

## Scope

**Incluye:**
- `Formatter` interface: `Format(w io.Writer, data any, opts FormatOpts) error`
- `TableFormatter`: columnas auto-detectadas de struct tags, headers en bold, colores por tipo
- `JSONFormatter`: `json.MarshalIndent` con indentación 2 espacios
- `YAMLFormatter`: `gopkg.in/yaml.v3` marshaling
- TTY detection: `isatty.IsTerminal(os.Stdout.Fd())`
- Pipe detection: default a JSON cuando no es TTY
- Colorized output: `fatih/color` o `gookit/color`
- Spinner: `theckman/yacspin` o similar para operaciones largas
- `--output` flag global con autocomplete: table, json, yaml

**Excluye:**
- Output a archivo (`> file` se maneja con shell redirect)
- Formato CSV (futuro)
- Pretty-print para JSON/YAML (ya lo hacen)

## Enfoque técnico

**Formatter interface:**
```go
type OutputFormat string

const (
    FormatTable OutputFormat = "table"
    FormatJSON  OutputFormat = "json"
    FormatYAML  OutputFormat = "yaml"
)

type Formatter interface {
    Format(w io.Writer, data any) error
}

type FormatterFactory struct {
    isTTY  bool
    format OutputFormat
}

func (f *FormatterFactory) Create() Formatter {
    if f.format == "" && !f.isTTY {
        return &JSONFormatter{}  // pipe → default json
    }
    switch f.format {
    case FormatTable: return &TableFormatter{color: f.isTTY}
    case FormatJSON:  return &JSONFormatter{}
    case FormatYAML:  return &YAMLFormatter{}
    default: return &TableFormatter{color: f.isTTY}
    }
}
```

**Table auto-column detection:**
```go
type TableFormatter struct {
    color bool
}

func (f *TableFormatter) Format(w io.Writer, data any) error {
    rows := reflect.ValueOf(data)
    // Si es slice, usar elementos como rows
    // Detectar columnas de struct tags `json:"name"` o `table:"Name"`
    // Renderizar con tablewriter (github.com/olekukonez/tablewriter)
}
```

**Spinner usage:**
```go
type Spinner struct {
    spinner *yacspin.Spinner
}

func NewSpinner( text string) *Spinner {
    cfg := yacspin.Config{
        Frequency: 100 * time.Millisecond,
        CharSet:   yacspin.CharSets[11], // dots
        Message:   text,
    }
    s, _ := yacspin.New(cfg)
    return &Spinner{spinner: s}
}
```

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| Colores ANSI en Windows | Usar librería cross-platform (fatih/color soporta Windows) |
| Spinner bloquea output real | Ocultar spinner antes de imprimir resultado final |
| Struct tags cambian entre entidades | Consistent naming convention en todas las entidades |
| Pipe detection en tests unitarios | Mockear isatty en tests |

## Testing

- Unit: cada formatter con datos mock
- Integration: CLI output comparado contra golden files
- TTY vs pipe: test con stdout pipe y terminal mock
- Spinner: verificar que no bloquea
