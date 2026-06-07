# HU-16.5-web-admin-memories

**Origen:** `REQ-16-web-ui`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de Domain
**Quiero** buscar, visualizar, editar y exportar las memorias del proyecto desde una interfaz web
**Para** revisar y gestionar el conocimiento acumulado sin usar la CLI

## Criterios de aceptación

### Escenario 1: Buscar memorias

```gherkin
Dado un proyecto con 500 observaciones
Cuando voy a /memories y busco "bug login"
Entonces veo resultados paginados con snippet de contenido destacado
Y cada resultado muestra: título, tipo, autor, fecha, score de relevancia
```

### Escenario 2: Ver detalle de memoria

```gherkin
Dado que hago click en una memoria de tipo "decision"
Entonces veo: título, contenido completo, tipo, proyecto, scope, autor, fecha de creación/actualización
Y veo memorias relacionadas (por tipo y proyecto)
Y veo el timeline contextual (antes/después de esta memoria)
```

### Escenario 3: Editar memoria

```gherkin
Dado que veo el detalle de una memoria
Cuando hago click en "Editar"
Y modifico el contenido
Entonces se actualiza la memoria
Y se registra en activity_log
```

### Escenario 4: Exportar memorias

```gherkin
Cuando hago click en "Exportar"
Entonces veo opciones de formato: JSON, Markdown, CSV
Y elijo exportar por proyecto y rango de fechas
Y descargo un archivo con todas las memorias seleccionadas
```

### Escenario 5: Ver memorias por usuario

```gherkin
Dado que Juan, Mauricio y Ariel trabajan en el mismo proyecto
Cuando voy a /memories y filtro por autor="Juan"
Entonces veo solo las memorias creadas por Juan
Y cada memoria muestra el avatar/nombre del autor
```

## Análisis breve

- **Qué pide realmente:** UI web para búsqueda full-text de memorias con filtros (autor, tipo, proyecto, fecha), edición inline, exportación, timeline contextual
- **Riesgos / dependencias:** Depende de REQ-03 (memory system)
- **Esfuerzo tentativo:** M
