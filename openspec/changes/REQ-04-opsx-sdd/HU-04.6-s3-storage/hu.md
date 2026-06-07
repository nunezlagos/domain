# HU-04.6-s3-storage

**Origen:** `REQ-04-opsx-sdd`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** equipo de desarrollo documentando requisitos y diseños en Domain
**Quiero** poder adjuntar imágenes, diagramas y archivos a las HUs, REQs y knowledge docs, almacenados en S3 (o compatible)
**Para** enriquecer la documentación visualmente sin depender de URLs externas frágiles

## Criterios de aceptación

### Escenario 1: Subir archivo a S3

```gherkin
Dado que configuro DOMAIN_S3_BUCKET=domain-assets y DOMAIN_S3_REGION=us-east-1
Cuando subo un archivo "diagrama.png" (2MB) asociado a la HU "HU-04.1"
Entonces el archivo se almacena en S3 bajo la key "attachments/hu-04.1/diagrama.png"
Y se guarda un registro en la tabla `file_attachments`
Y la respuesta incluye la URL firmada (signed URL) para acceso temporal
```

### Escenario 2: Obtener archivo adjunto

```gherkin
Dado que existe un attachment con id "att-abc" en la HU "HU-04.1"
Cuando consulto GET /api/v1/attachments/att-abc/download
Entonces devuelve una signed URL de S3 válida por 1 hora
```

### Escenario 3: Adjuntar a knowledge docs

```gherkin
Dado un knowledge_doc con id "kd-123"
Cuando subo un archivo "arquitectura.pdf" asociado a "kd-123"
Entonces el archivo se almacena en S3 bajo "knowledge/kd-123/arquitectura.pdf"
Y el knowledge_doc queda marcado como "tiene_adjuntos = true"
```

### Escenario 4: Soporte multi-provider S3

```gherkin
Dado que configuro DOMAIN_S3_ENDPOINT=https://minio.internal:9000
Y DOMAIN_S3_REGION=us-east-1
Y DOMAIN_S3_ACCESS_KEY y DOMAIN_S3_SECRET_KEY
Cuando subo un archivo
Entonces se almacena en MinIO usando la misma API S3
```

### Escenario 5: Límite de tamaño y tipo

```gherkin
Dado que el límite es 10MB por archivo
Cuando intento subir un archivo de 15MB
Entonces recibo error "file too large: max 10MB"

Dado que los tipos permitidos son imagen, PDF, markup
Cuando intento subir un ".exe"
Entonces recibo error "file type not allowed"
```

### Escenario 6: Cleanup de adjuntos huérfanos

```gherkin
Dado que un attachment existe en S3 pero su entidad padre fue eliminada
Cuando ejecuta el cleanup diario
Entonces el archivo se elimina de S3
Y el registro en file_attachments se elimina
```

## Análisis breve

- **Qué pide realmente:** Integración con S3-compatible storage (AWS S3, MinIO, R2, DO Spaces) para adjuntos. Tabla `file_attachments` con: id, entity_type, entity_id, filename, s3_key, size, mime_type, created_by. Signed URLs para acceso temporal. Cleanup de huérfanos.
- **Módulos sospechados:** `internal/store/s3/`, `internal/service/attachment.go`
- **Riesgos / dependencias:** Dependencia externa de S3. Costos de storage. Signed URLs expiran. Archivos huérfanos si se elimina la entidad padre.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
