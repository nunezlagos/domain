# Tasks: issue-14.2-cli-output-formats

## Backend

- [x] Definir `Formatter` interface con `Format(w io.Writer, data any) error`
- [x] Implementar `TableFormatter` con auto-detección de columnas via reflect
- [x] Implementar `JSONFormatter` con indentación
- [x] Implementar `YAMLFormatter` con gopkg.in/yaml.v3
- [x] Implementar `FormatterFactory` con detección TTY y flag --output
- [x] Integrar formato como flag global de rootCmd
- [x] Integrar formatters en todos los subcomandos list/get
- [x] Implementar colorización de tabla en TTY (por tipo de entidad)
- [x] Implementar truncado de campos largos en tabla
- [x] Implementar Spinner para operaciones largas (agent run, flow execute)
- [x] Spinner solo visible en TTY
- [x] Formatear fechas en formato legible para tablas

## Frontend

- [x] N/A (CLI tool)

## Tests

- [x] Test unitario: TableFormatter con datos mock
- [x] Test unitario: JSONFormatter produce JSON válido
- [x] Test unitario: YAMLFormatter produce YAML válido
- [x] Test unitario: TTY detection mockeada
- [x] Test unitario: --output inválido da error
- [x] Test de integración: CLI output comparado con golden files
- [x] Test de integración: pipe vs TTY detection
- [x] Sabotaje: formatter que no usa columnas correctas → test detecta

## Cierre

- [x] Verificación manual: `domain memory list --output json | jq`
- [x] Verificación manual: colores en terminal vs pipe
- [x] Suite verde: `go test ./internal/cli/...`
