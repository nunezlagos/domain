# issue-54.1 — Tasks

## Implementación

- [ ] cambiar domain_code_graph de ChannelHook a ChannelUserIntent en tool_channels.go + comentario de deprecación
- [ ] actualizar TOOL_CHANNELS.md: mover code_graph a user-intent, agregar domain_issue_list y domain_flow_cancel
- [ ] actualizar conteos del header del doc (hook 5→4, user-intent 95→98)
- [ ] ajustar el header del doc (de "GENERADO — NO editar" a "mantenido sincronizado, validado por test")

## Tests

- [ ] TestToolChannelsDocInSync: parsea TOOL_CHANNELS.md y verifica que refleja el map toolChannel (tools + canales) bidireccional
- [ ] verificar que TestAllToolsHaveChannel y TestChannelDistribution siguen verdes tras el cambio de canal
- [ ] Sabotaje: quitar una tool del doc → TestToolChannelsDocInSync falla

## Documentación

- [ ] Actualizar CHANGELOG Unreleased
- [ ] Actualizar state.yaml a implemented al cerrar
