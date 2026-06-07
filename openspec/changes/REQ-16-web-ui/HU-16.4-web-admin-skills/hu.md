# HU-16.4-web-admin-skills

**Origen:** `REQ-16-web-ui`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** desarrollador usando la plataforma Domain
**Quiero** poder crear, editar, probar y versionar skills desde una interfaz web
**Para** gestionar los skills de mi organización sin usar la CLI

## Criterios de aceptación

### Escenario 1: Listar skills

```gherkin
Dado que existen 10 skills en la organización
Cuando navego a /skills
Entonces veo una tabla con: nombre, descripción, versión, estado, última ejecución
Y puedo filtrar por nombre y ordenar por fecha de creación
```

### Escenario 2: Crear skill desde web

```gherkin
Cuando hago click en "Nuevo skill"
Entonces veo un formulario con: nombre, descripción, tipo (prompt/code/API), contenido
Y al guardar, el skill se crea con versión 1.0.0
Y veo el detalle del skill creado
```

### Escenario 3: Editar skill

```gherkin
Dado un skill existente "code-review"
Cuando edito su contenido prompt
Y guardo los cambios
Entonces se crea una nueva versión (1.0.0 → 1.1.0)
Y veo el diff entre la versión anterior y la nueva
```

### Escenario 4: Probar skill

```gherkin
Dado el skill "code-review" con versión 1.0.0
Cuando hago click en "Probar"
Y veo un panel con parámetros de entrada
Y ejecuto el skill
Entonces veo el resultado en tiempo real
Y veo el tiempo de ejecución y tokens consumidos
```

### Escenario 5: Ver historial de versiones

```gherkin
Dado un skill con 5 versiones
Cuando voy a /skills/code-review/versions
Entonces veo un timeline con todas las versiones
Y puedo ver el contenido de cualquier versión anterior
Y puedo restaurar una versión anterior (crea nueva versión con ese contenido)
```

## Análisis breve

- **Qué pide realmente:** UI web completa para CRUD de skills, versionado visual, diff entre versiones, panel de prueba con resultados en vivo
- **Riesgos / dependencias:** Depende de REQ-05 (skill system) y HU-06.6 (streaming)
- **Esfuerzo tentativo:** L
