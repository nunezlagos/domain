# HU-05.3-skill-versioning

**Origen:** `REQ-05-skill-system`
**Persona:** dx-engineer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador de la plataforma
**Quiero** que los skills tengan versionado completo: cada actualización crea una nueva versión, poder pinchar a una versión específica, hacer rollback, y ver diferencias entre versiones
**Para** tener trazabilidad y control sobre los cambios en skills compartidos

## Criterios de aceptación

### Escenario 1: Actualizar skill crea nueva versión

```gherkin
Dado que existe un skill "sk-abc-123" con version=1 y content="version1"
Cuando PATCH /api/skills/sk-abc-123 con:
  """
  { "content": "version2" }
  """
Entonces recibo 200 OK
Y el campo `version` del skill es 2
Y `content` es "version2"
Y existe un registro en skill_versions con version=1, content="version1"
```

### Escenario 2: Obtener versión específica de un skill

```gherkin
Dado que el skill "sk-abc-123" tiene versiones 1, 2 y 3
Cuando GET /api/skills/sk-abc-123/versions/1
Entonces recibo 200 OK
Y el body contiene el content y metadata exactos de la versión 1
```

### Escenario 3: Listar historial de versiones

```gherkin
Dado que el skill "sk-abc-123" tiene 5 versiones
Cuando GET /api/skills/sk-abc-123/versions
Entonces recibo 200 OK
Y el array `data` contiene 5 items
Y cada item tiene: version, content_snippet, changelog, created_at, author_id
Y están ordenados por version DESC
```

### Escenario 4: Pin skill a versión específica

```gherkin
Dado que existe un skill con versiones 1, 2, 3
Cuando pincho una skill referencia a versión 1
  POST /api/skills/sk-abc-123/pin
  { "version": 1 }
Entonces recibo 200 OK
Y GET /api/skills/sk-abc-123 retorna el content de la versión 1
Y `pinned_version` es 1
```

### Escenario 5: Rollback a versión anterior

```gherkin
Dado que el skill "sk-abc-123" está en version=5 con contenido erróneo
Cuando POST /api/skills/sk-abc-123/rollback con:
  """
  { "target_version": 3 }
  """
Entonces recibo 200 OK
Y `version` es 6 (nueva versión)
Y `content` es idéntico al de la versión 3
Y el changelog indica "rollback to version 3"
```

### Escenario 6: Diff entre dos versiones

```gherkin
Dado que el skill tiene versiones 2 y 5
Cuando GET /api/skills/sk-abc-123/diff?from=2&to=5
Entonces recibo 200 OK
Y el body contiene:
  """
  {
    "from": 2,
    "to": 5,
    "changes": [
      { "field": "content", "type": "modified", "diff": "@@ -1,3 +1,4 @@\n ..." },
      { "field": "parameters.properties.temperature", "type": "added" }
    ],
    "breaking": false
  }
  """
```

### Escenario 7: Marcar versión como breaking change

```gherkin
Dado que actualizo un skill cambiando parámetros requeridos
Cuando PATCH /api/skills/sk-abc-123 con:
  """
  {
    "content": "nuevo contenido",
    "parameters": {
      "type": "object",
      "properties": { "new_required_field": { "type": "string" } },
      "required": ["new_required_field"]
    },
    "breaking": true
  }
  """
Entonces recibo 200 OK
Y skill_versions registra `breaking = true` para esta versión
Y los skills que referencian este skill con `pinned_version` reciben una notificación de breaking change
```

### Escenario 8: Usar versión latest

```gherkin
Dado que un skill tiene versiones 1..5
Cuando otro skill lo referencia sin `pinned_version`
Entonces al ejecutar se usa la versión más reciente (5)
Y si se crea la versión 6, las ejecuciones futuras usan la 6 automáticamente
```

### Escenario 9: No se puede hacer rollback a versión inexistente

```gherkin
Dado que el skill tiene versiones 1..3
Cuando POST /api/skills/sk-abc-123/rollback con target_version=99
Entonces recibo 404 Not Found
Y el mensaje indica "version 99 does not exist"
```

## Análisis breve

- **Qué pide realmente:** Versionado completo de skills con tabla `skill_versions`, operaciones de pin, rollback, diff estructural, y detección de breaking changes.
- **Módulos sospechados:** `internal/skill/version.go`, `internal/database/migrations/`, `internal/api/handlers/skill_version.go`
- **Riesgos / dependencias:** Depende de HU-05.1 (tabla skills). El diff necesita un algoritmo de diff estructurado (JSON-aware, no solo texto plano).
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
