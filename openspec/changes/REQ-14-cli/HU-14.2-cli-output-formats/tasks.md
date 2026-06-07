# Tasks: HU-14.2-cli-output-formats

## Backend

- [ ] Definir `Formatter` interface con `Format(w io.Writer, data any) error`
- [ ] Implementar `TableFormatter` con auto-detección de columnas via reflect
- [ ] Implementar `JSONFormatter` con indentación
- [ ] Implementar `YAMLFormatter` con gopkg.in/yaml.v3
- [ ] Implementar `FormatterFactory` con detección TTY y flag --output
- [ ] Integrar formato como flag global de rootCmd
- [ ] Integrar formatters en todos los subcomandos list/get
- [ ] Implementar colorización de tabla en TTY (por tipo de entidad)
- [ ] Implementar truncado de campos largos en tabla
- [ ] Implementar Spinner para operaciones largas (agent run, flow execute)
- [ ] Spinner solo visible en TTY
- [ ] Formatear fechas en formato legible para tablas

## Frontend

- [ ] N/A (CLI tool)

## Tests

- [ ] Test unitario: TableFormatter con datos mock
- [ ] Test unitario: JSONFormatter produce JSON válido
- [ ] Test unitario: YAMLFormatter produce YAML válido
- [ ] Test unitario: TTY detection mockeada
- [ ] Test unitario: --output inválido da error
- [ ] Test de integración: CLI output comparado con golden files
- [ ] Test de integración: pipe vs TTY detection
- [ ] Sabotaje: formatter que no usa columnas correctas → test detecta

## Cierre

- [ ] Verificación manual: `domain memory list --output json | jq`
- [ ] Verificación manual: colores en terminal vs pipe
- [ ] Suite verde: `go test ./internal/cli/...`
