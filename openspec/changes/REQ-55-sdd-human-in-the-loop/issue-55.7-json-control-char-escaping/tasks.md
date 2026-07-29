# issue-55.7 json-control-char-escaping — Tasks
- [ ] grep de construcción manual de tool result text (fmt.Sprintf/concatenación) en internal/mcp/server/
- [ ] Reemplazar por json.Marshal / toolResultJSON / mcp.NewToolResultText
- [ ] Test Go: tool result con \n/\t/comillas → JSON parseable estricto
- [ ] Verificar en vivo: los tools que fallaban en la sesión ahora parsean sin strict=False
