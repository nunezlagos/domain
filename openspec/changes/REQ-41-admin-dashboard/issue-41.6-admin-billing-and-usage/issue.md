# issue-41.6-admin-billing-and-usage

**Origen:** `REQ-41-admin-dashboard`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** admin de una organización
**Quiero** ver mi plan actual, el consumo vs límites de cada dimensión, alertas activas y opciones de upgrade
**Para** entender cuánto estoy usando, evitar cortes por exceso y tomar decisiones de upgrade

## Criterios de aceptación

```gherkin
Feature: Billing & Usage

  Background:
    Given el usuario está autenticado con rol admin/owner
    And la org tiene un plan asignado (Free / Pro / Enterprise)

  Scenario: Vista de plan actual
    When navego a /admin/billing
    Then veo el plan actual con CardComponent destacado:
      | campo | valor |
      | Plan name | "Pro" (o Free/Enterprise) |
      | Precio | "$199/mes" (o "Gratis" / "Custom") |
      | Renovación | fecha del próximo ciclo |
      | Método de pago | Visa **** 4242 (o "Sin método — agregar") |

  Scenario: Tabla de uso vs límites
    When veo la sección "Uso del plan"
    Then veo una tabla con barras ProgressComponent:
      | dimensión         | ejemplo Free     | ejemplo Pro        |
      | Tokens/mes        | 100k / 500k (20%)| 1.2M / 5M (24%)    |
      | Runs/mes          | 50 / 1k (5%)     | 4.5k / 10k (45%)   |
      | Storage GB        | 2 / 5 (40%)      | 12 / 50 (24%)      |
      | Members           | 2 / 3 (66%)      | 12 / 25 (48%)      |
    Y la barra se pone amarilla si >80%, roja si >100%
    Y cada dimensión tiene tooltip con desglose histórico (últimos 30 días)

  Scenario: Alertas activas
    When hay usage_alerts configuradas
    Then veo una CardComponent con lista de alertas:
      | nombre | condición | estado |
      | "Tokens altos" | > 80% tokens/mes | "Activa" / "Disparada" |
      | "Storage full" | > 90% storage | "Inactiva" |
    Y cada una tiene botón "Editar" / "Pausar" / "Eliminar"

  Scenario: Crear alerta
    When hago clic en "Nueva alerta"
    Then se abre ModalComponent con form:
      | campo | opciones |
      | Nombre | text |
      | Dimensión | select (tokens, runs, storage, members) |
      | Umbral | número 0-100 (%) |
      | Canal de notificación | select (email, slack, webhook) |
    Y al guardar, llama POST /api/v1/usage-alerts

  Scenario: Upgrade CTA (cuando hay Stripe)
    Given el plan actual es Free o Pro
    When hago clic en "Upgrade"
    Then se abre un ModalComponent con opciones: Free → Pro, Pro → Enterprise
    Y al confirmar, llama POST /api/v1/billing/checkout (issue-21.4)
    Y redirige a Stripe Checkout
    Y al volver del success, se actualiza el plan

  Scenario: Sin Stripe configurado
    Given DOMAIN_STRIPE_SECRET no está configurado
    When veo /admin/billing
    Then el botón "Upgrade" está disabled con tooltip "Stripe no configurado en esta instancia"
    Y se muestra un AlertComponent info "Para habilitar billing, configurá Stripe en el .env"

  Scenario: Histórico de invoices
    Given hay invoices pasadas
    When hago clic en "Ver invoices"
    Then se abre ModalComponent con tabla: fecha, número, monto, status (paid/pending/failed), link al PDF
    Y un botón "Descargar todo" descarga un zip

  Scenario: Estimación de cost del mes en curso
    When veo la sección "Cost estimado"
    Then veo: "Este mes vas a gastar aprox. $X.XX (basado en uso al día de hoy)"
    Y una proyección: "Si seguís así, el total del mes será ~$Y.YY"

  Scenario: Empty state
    Given la org no tiene plan asignado (recién creada)
    When veo /admin/billing
    Then veo AlertComponent warning "No tenés un plan asignado. Contactanos o self-assign Free."
```

## Componentes del template CoreUI a reusar

| Componente | Path | Uso |
|---|---|---|
| `CardComponent` + `CardBodyComponent` + `CardHeaderComponent` | `views/base/cards/` | Plan card, Usage card, Alerts card, Invoices card, Cost estimado card |
| `ProgressComponent` | `views/base/progress/` | Barras de uso (con variant success/warning/danger) |
| `TableDirective` | `views/base/tables/` | Tabla de uso, tabla de invoices, tabla de alertas |
| `BadgeComponent` | `views/notifications/badges/` | Status badges (Activa/Disparada/Pagada/Pendiente) |
| `ModalComponent` + set completo | `views/notifications/modal/` | Modales de upgrade, crear alerta, ver invoices |
| `FormControlDirective` + `FormLabelDirective` | `views/forms/form-control/` | Inputs de alertas |
| `FormSelectDirective` | `views/forms/select/` | Selectores (dimensión, canal) |
| `FormCheckComponent` | `views/forms/checks-radios/` | Toggles de notificaciones |
| `ButtonDirective` | `views/buttons/` | CTAs (Upgrade, Nueva alerta, Editar) |
| `ButtonGroup` | `views/buttons/button-groups/` | Toggle entre "Uso / Invoices / Alertas" |
| `AlertComponent` | `views/notifications/alerts/` | Warnings, info de Stripe no configurado |
| `SpinnerComponent` | `views/base/spinners/` | Loading al cambiar plan o crear alerta |
| `ToasterComponent` | `views/notifications/toasts/` | Feedback de éxito/error |
| `Tabs` (Navs & Tabs) | `views/base/navs/` | Tabs principales (Plan / Uso / Alertas / Invoices) |
| `IconDirective` (cil-*) | `@coreui/angular` | Iconos (`cil-credit-card`, `cil-chart`, `cil-bell`, `cil-warning`, `cil-dollar`) |

## Endpoints del backend

| Endpoint | Acción | Estado |
|---|---|---|
| `GET /api/v1/usage` | Plan + usage actual (issue-21.3) | ya existe |
| `GET /api/v1/usage/current` | Snapshot actual de usage | ya existe |
| `GET /api/v1/usage/history` | Histórico de usage | ya existe |
| `GET /api/v1/usage-alerts` | Lista de alertas | ya existe |
| `POST /api/v1/usage-alerts` | Crear alerta | ya existe |
| `PATCH /api/v1/usage-alerts/{id}` | Editar/pausar alerta | ya existe |
| `DELETE /api/v1/usage-alerts/{id}` | Eliminar alerta | ya existe |
| `GET /api/v1/usage-alerts/{id}/fires` | Historial de disparos | ya existe |
| `GET /api/v1/billing/invoices` | Lista de invoices | **VERIFICAR**; depende de issue-21.4 (Stripe) |
| `POST /api/v1/billing/checkout` | Inicia Stripe Checkout | **VERIFICAR**; depende de issue-21.4 |
| `POST /api/v1/billing/portal` | Abre Stripe Customer Portal | **VERIFICAR**; depende de issue-21.4 |
| `GET /api/v1/cost/spend/{granularity}` | Cost por día/semana/mes | ya existe (issue-15) |
| `GET /api/v1/cost/forecast` | Proyección de cost | ya existe (issue-15) |

**Nuevos a crear en esta HU** (si hicieran falta):
- Probablemente ninguno — todo el backend ya está planeado en REQ-15 + REQ-21. Si issue-21.4 (Stripe) no está implementado, la vista de billing muestra "próximamente" en las secciones de upgrade/invoices.

## Análisis breve

- **Qué pide realmente:** Vista de plan + uso + alertas + (futuro) upgrade vía Stripe. Hoy depende mucho de REQ-21.4 (Stripe) que puede no estar listo.
- **Módulos a tocar:** Nueva vista `views/admin-billing/`. Si todo el backend existe, es solo frontend.
- **Riesgos / dependencias:** Si issue-21.4 (Stripe) no está implementado, la vista de billing es degradada: muestra plan/uso/alertas pero upgrade está disabled. Decidir si lanzar HU-41.6 antes o después de REQ-21.4.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Confirmar que `GET /api/v1/usage` devuelve `{plan, limits, usage}` con todas las dimensiones
- [ ] Verificar si `GET /api/v1/billing/invoices` existe o depende de issue-21.4
- [ ] Confirmar que `usage-alerts` soporta los canales email/slack/webhook
- [ ] Decidir: ¿lanzar HU-41.6 antes o después de REQ-21.4?
- [ ] Confirmar que `cost/forecast` devuelve proyección con horizonte mensual
- [ ] Validar que el upgrade CTA está bien disabled cuando `DOMAIN_STRIPE_SECRET` no existe

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
