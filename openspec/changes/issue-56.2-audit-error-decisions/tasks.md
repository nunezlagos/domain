# Toda clasificación de error como benigno (known_error_set) y todo borrado (error_reset) queda auditado con actor, tiempo y razón; error_reset es soft-delete reversible; y los hooks lifecycle registran sus propios fallos en vez de silenciarlos, dejando trazabilidad completa de las decisiones sobre errores. — Tasks

