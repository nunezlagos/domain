# Design: HU-14.2-cli-output-formats

## Decisión arquitectónica

**Strategy pattern:** `Formatter` interface con implementaciones intercambiables. La fábrica elige la implementación según `--output` flag y detección de TTY.

**Flujo de output:**
```
Command.RunE → obtiene data de API → FormatterFactory.Create() → Formatter.Format(stdout, data)
```

**Table formatter internals:**
- Usa `github.com/olekukonez/tablewriter` para renderizado de tablas
- Columnas se detectan via tags `json:"field_name"` en los structs
- Encabezados en mayúscula: "ID", "Title", "Type", "Created At"
- Colores por valor de `type` field: fix=red, decision=blue, context=green, prompt=yellow
- Alternating row colors para legibilidad
- Truncar campos largos (>80 chars) con "..."
- Fechas en formato legible: "Jan 02, 2006 15:04 UTC"

**JSON formatter:**
```go
func (f *JSONFormatter) Format(w io.Writer, data any) error {
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    enc.SetEscapeHTML(false)
    return enc.Encode(data)
}
```

**YAML formatter:**
```go
func (f *YAMLFormatter) Format(w io.Writer, data any) error {
    out, err := yaml.Marshal(data)
    if err != nil {
        return err
    }
    _, err = w.Write(out)
    return err
}
```

**TTY detection:**
```go
import "github.com/mattn/go-isatty"

func isTTY() bool {
    return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}
```

## Alternativas descartadas

1. **Solo JSON siempre:** Menos amigable para humanos en terminal. Tabla es más legible para listados.
2. **Usar templates Go (text/template):** Muy verboso, requiere template por comando. El formatter automático es más mantenible.
3. **No detectar pipe:** Forzar --output siempre, UX pobre. La detección automática mejora la experiencia.

## Diagrama

```
Command
  │
  ▼
┌────────────────────┐
│  Get data from API  │
│  (list entities)    │
└────────┬───────────┘
         ▼
┌────────────────────┐
│  FormatterFactory   │
│  ├── isTTY?         │
│  ├── --output flag  │
│  └── choose impl    │
└────────┬───────────┘
         ▼
┌──────────────────────┐
│  TableFormatter       │ ← TTY + table
│  JSONFormatter        │ ← pipe o --output json
│  YAMLFormatter        │ ← --output yaml
└──────────┬───────────┘
           ▼
     os.Stdout
```

## TDD plan

1. **Red:** Test `TestTableFormatter` con lista de observaciones mock
2. **Green:** Implementar `TableFormatter` básico
3. **Refactor:** Extraer interface, factory
4. **Iterar:** JSONFormatter, YAMLFormatter, color, TTY detection
5. **Sabotaje:** Formatter que no escribe nada → test detecta

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Columnas inconsistentes entre entidades | Naming convention + test que verifica tags en todos los structs |
| Spinner visible en output pipeado | Solo mostrar si isTTY() |
| Tabla con muchas columnas se ve mal | tablewriter con wrapping, o permitir `--output json` para datos complejos |
