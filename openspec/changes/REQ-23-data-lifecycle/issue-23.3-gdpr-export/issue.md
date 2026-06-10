# issue-23.3-gdpr-export

**Origen:** `REQ-23-data-lifecycle`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario sujeto a GDPR / privacy
**Quiero** exportar todos mis datos en formato portable (ZIP con JSON + adjuntos)
**Para** ejercer derecho de portabilidad y respaldo personal

## Criterios de aceptación

### Escenario 1: Solicitar export

```gherkin
Dado que soy un user autenticado
Cuando POST /api/v1/me/export
Entonces se crea job en tabla `export_jobs` con status pending
Y se devuelve job_id
Y se notifica via email cuando esté listo
```

### Escenario 2: Contenido del export

```gherkin
Dado que job termina
Cuando descargo `export.zip`
Entonces contiene:
  | path                   | contenido                         |
  | manifest.json          | metadata: user_id, export_date, version |
  | profile.json           | datos del user                    |
  | organizations.json     | orgs donde el user es miembro     |
  | projects.json          | projects accesibles                |
  | observations.json      | observations creadas por el user   |
  | sessions.json          | sessions del user                  |
  | prompts.json           | prompts del user                   |
  | knowledge_docs.json    | knowledge del user                 |
  | agent_runs.json        | runs disparados por el user        |
  | attachments/<files>    | adjuntos referenciados             |
  | README.md              | explicación del formato            |
```

### Escenario 3: Signed URL temporal

```gherkin
Dado que el ZIP está en S3
Cuando el job está listo
Entonces se genera signed URL con TTL 24h
Y se incluye en el email de notificación
Y descarga es accesible solo con esa URL
```

### Escenario 4: Privacidad — solo datos del solicitante

```gherkin
Dado que el user pertenece a org compartida
Cuando se exporta
Entonces solo se incluyen entidades creadas por o asignadas al user
Y NO se incluyen datos de otros miembros aunque sean visibles
```

### Escenario 5: Performance

```gherkin
Dado que el user tiene ~10k observations
Cuando se procesa el export
Entonces termina en menos de 5 minutos
Y el ZIP no excede 1 GB para cuenta promedio
```

### Escenario 6: Rate limit

```gherkin
Dado que un user solicita export
Cuando intenta solicitar otro export en <24h
Entonces 429 "export already requested in last 24h"
Y se devuelve el job_id del anterior
```

## Análisis breve

- **Qué pide:** job async + serialización JSON + ZIP + S3 upload + signed URL + email notif
- **Esfuerzo:** M
- **Riesgos:** datos PII de otros usuarios filtrados; tamaño ZIP enorme; performance
