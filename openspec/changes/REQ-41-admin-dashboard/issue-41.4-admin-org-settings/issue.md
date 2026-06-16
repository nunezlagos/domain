# issue-41.4-admin-org-settings

**Origen:** `REQ-41-admin-dashboard`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** admin de una organización
**Quiero** ver y editar los settings de mi org (nombre, timezone, default model, default channel, branding básico)
**Para** personalizar el comportamiento por defecto sin tener que editar variables de entorno

## Criterios de aceptación

```gherkin
Feature: Org Settings

  Background:
    Given el usuario está autenticado con rol admin/owner

  Scenario: Vista de settings con secciones colapsables
    When navego a /admin/settings
    Then veo la página organizada en Tabs (o Accordion):
      | tab            | contenido editable |
      | General        | name, slug, description, timezone, default_locale |
      | AI defaults    | default_model, default_provider, max_tokens_per_request, default_temperature |
      | Notifications  | default_channel, webhook URL para eventos críticos |
      | Branding       | logo_url, primary_color, accent_color (preview en vivo) |
      | Danger zone    | delete org (solo owner) |

  Scenario: Editar settings generales
    When edito name="Acme AI" y timezone="America/Santiago"
    And guardo
    Then llama PATCH /api/v1/organizations/{id} con body {name, timezone}
    And veo toast "Settings guardados"
    And el header refleja el nuevo name

  Scenario: Slug es inmutable una vez creado
    When veo el campo slug
    Then está disabled con tooltip "El slug no se puede cambiar (afecta URLs)"
    And el icono de candado es visible

  Scenario: Editar AI defaults
    When cambio default_model="gpt-4o" y guardo
    Then llama PATCH /api/v1/organizations/{id} con {default_model, default_provider}
    And se aplica a los nuevos agentes que se creen (no afecta los existentes)
    Y veo toast de éxito

  Scenario: Branding con preview en vivo
    When subo un logo (drag & drop o file picker)
    Then se muestra preview inmediato en el header
    And al guardar, sube a MinIO y persiste logo_url
    And un cambio de primary_color aplica via CSS variable en el preview

  Scenario: Danger zone — eliminar org (solo owner)
    Given soy owner de la org
    When hago clic en "Eliminar organización"
    Then se abre ModalComponent con doble check ("escribí el nombre de la org")
    Y advierto: se borran TODOS los proyectos, agentes, flows, memories, runs
    Y se revocan TODAS las API keys y sesiones
    Y se preserva el audit log (exportable)
    And al confirmar, llama DELETE /api/v1/organizations/{id}
    And redirige a /login con mensaje "Org eliminada"

  Scenario: Sin permisos (developer o viewer)
    Given soy developer/viewer
    When navego a /admin/settings
    Then el authGuard de la ruta redirige a /admin/dashboard
    And ningún campo se muestra editable

  Scenario: Validación de form
    When dejo name vacío
    Then el botón "Guardar" está disabled
    And el campo tiene mensaje "Requerido" (FormControlDirective validación)

  Scenario: Cambios sin guardar
    When edito name y navego a otra tab sin guardar
    Then aparece ModalComponent de confirmación "¿Descartar cambios?"
    And opciones: "Volver y guardar" / "Descartar" / "Cancelar"
```

## Componentes del template CoreUI a reusar

| Componente | Path | Uso |
|---|---|---|
| `Tabs` (o `Navs & Tabs`) | `views/base/tabs/` | Tabs principales (General / AI defaults / Notifications / Branding / Danger zone) |
| `AccordionComponent` | `views/base/accordion/` | Alternativa a tabs para vista colapsable |
| `FormControlDirective` + `FormLabelDirective` + `FormTextDirective` | `views/forms/form-control/` | Inputs de texto |
| `FormSelectDirective` | `views/forms/select/` | Selectores (timezone, default_model, default_provider) |
| `FormCheckComponent` + `FormCheckInputDirective` | `views/forms/checks-radios/` | Toggles (enable webhook, etc.) |
| `FloatingLabels` | `views/forms/floating-labels/` | Inputs con label flotante (alternativa más bonita) |
| `InputGroupComponent` | `views/forms/input-groups/` | Slug con prefijo readonly (e.g. `https://app.domain.com/orgs/[slug-disabled]`) |
| `Validation` patterns | `views/forms/validation/` | Mensajes de error por campo |
| `ModalComponent` + set completo | `views/notifications/modal/` | Modal de "descartar cambios", "eliminar org" |
| `ButtonDirective` | `views/buttons/` | Guardar / Cancelar / Eliminar |
| `CardComponent` + `CardBodyComponent` + `CardHeaderComponent` | `views/base/cards/` | Wrapper de cada tab section |
| `AlertComponent` | `views/notifications/alerts/` | Warnings de "danger zone" |
| `SpinnerComponent` | `views/base/spinners/` | Loading al guardar |
| `ToasterComponent` | `views/notifications/toasts/` | Feedback de éxito/error |
| `IconDirective` (cil-*) | `@coreui/angular` | Iconos (`cil-settings`, `cil-palette`, `cil-warning`, `cil-lock-locked`) |

## Endpoints del backend

| Endpoint | Acción | Estado |
|---|---|---|
| `GET /api/v1/organizations/{id}` | Cargar settings actuales | ya existe |
| `PATCH /api/v1/organizations/{id}` | Guardar cambios | ya existe |
| `DELETE /api/v1/organizations/{id}` | Eliminar org (owner only) | ya existe |
| `POST /api/v1/attachments` + `POST /api/v1/attachments/{id}/confirm` | Upload de logo | ya existe (REQ-04) |
| `GET /api/v1/llm/models` (o `admin/runtime-configs/llm.models`) | Lista de modelos disponibles para el select | **VERIFICAR** si existe |

**Nuevos a crear en esta HU** (si hicieran falta):
- `PATCH /api/v1/organizations/{id}/branding` con `{logo_url, primary_color, accent_color}` — **VERIFICAR** si el PATCH actual cubre branding. Si no, agregarlo al body del PATCH existente.
- Si `branding` no está en la tabla `organizations`, agregar migración.

## Análisis breve

- **Qué pide realmente:** Vista de settings con tabs, form de edición, preview en vivo para branding, y danger zone con confirmación.
- **Módulos a tocar:** Nueva vista `views/admin-settings/` con sub-componentes por tab. Backend: verificar/extender el `PATCH /organizations/{id}`.
- **Riesgos / dependencias:** El delete de org es DESTRUCTIVO. El backend debe hacer soft-delete + audit, no DELETE físico (verificar). El branding (color, logo) debe persistir y aplicarse al render del admin (CSS vars). El default_model debe existir en el model registry (REQ-06.4).
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Confirmar que el backend ya soporta todos los campos que queremos editar (name, slug, timezone, default_model, etc.)
- [ ] Verificar si branding (logo, colors) está en el schema de `organizations` o hay que migrar
- [ ] Confirmar que `DELETE /organizations/{id}` hace soft-delete y no DELETE físico
- [ ] Verificar la lista de modelos disponibles en el registry para popular el select
- [ ] Confirmar que solo el owner puede eliminar la org
- [ ] Decidir si el form de settings es un solo "Guardar todo" o "Guardar por tab"

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
