# HU-21.4-billing-stripe

**Origen:** `REQ-21-org-billing`
**Prioridad tentativa:** baja
**Tipo:** feature

## Historia de usuario

**Como** plataforma
**Quiero** integrar Stripe Checkout + Webhooks para upgrade/downgrade de plans
**Para** cobrar a usuarios Pro/Enterprise con manejo de métodos de pago, invoices y dunning

## Criterios de aceptación

### Escenario 1: Upgrade Free → Pro

```gherkin
Dado que org está en plan Free
Cuando POST /api/v1/billing/checkout-session con `{"plan_slug":"pro"}`
Entonces se crea Stripe Checkout Session
Y se devuelve URL para que admin pague
Y al volver con success_url se confirma el upgrade
Y `organizations.plan_id` apunta a Pro
Y se crea registro `subscriptions` con stripe_subscription_id
```

### Escenario 2: Webhook procesa eventos Stripe

```gherkin
Dado que existe endpoint POST /api/v1/billing/stripe-webhook
Cuando Stripe envía evento `invoice.paid`
Y la signature `Stripe-Signature` es válida
Entonces se logea evento, se actualiza `subscriptions.last_invoice_paid_at`
Y se reset counter ciclo facturación
```

### Escenario 3: Downgrade

```gherkin
Dado que org Pro hace downgrade a Free
Cuando POST /api/v1/billing/cancel con effective="end_of_period"
Entonces la subscription Stripe se cancela al fin de periodo
Y `organizations.plan_id` pasa a Free después del fin de periodo
Y se notifica al admin
```

### Escenario 4: Failed payment / dunning

```gherkin
Dado que invoice.payment_failed llega via webhook
Cuando se procesa
Entonces se notifica admin con CTA "actualizar método de pago"
Y después de 3 fallos se downgrade automáticamente a Free
Y se notifica al admin del downgrade
```

### Escenario 5: Customer portal

```gherkin
Dado que admin quiere actualizar método de pago
Cuando POST /api/v1/billing/portal-session
Entonces se crea Stripe Customer Portal session
Y se devuelve URL para que admin gestione facturación, métodos de pago, invoices
```

## Análisis breve

- **Qué pide:** Stripe SDK + Checkout + Webhooks + Customer Portal + subscriptions table
- **Esfuerzo:** L
- **Riesgos:** webhook signature; idempotency; tax/VAT por jurisdicción; PCI compliance (mitigado al no manejar PAN directamente)
