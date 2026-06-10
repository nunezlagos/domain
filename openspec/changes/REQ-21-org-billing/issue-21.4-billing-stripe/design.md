# Design: issue-21.4-billing-stripe

## Decisión arquitectónica

**Stripe SDK:** `github.com/stripe/stripe-go/v79`
**Modelo:** Checkout Sessions + Customer Portal (no Elements custom)
**Razón:** delega PCI scope a Stripe, UI consistente, menos código

## Schema

```sql
CREATE TABLE subscriptions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  stripe_customer_id VARCHAR(64) NOT NULL,
  stripe_subscription_id VARCHAR(64) UNIQUE NOT NULL,
  plan_id UUID NOT NULL REFERENCES plans(id),
  status VARCHAR(30) NOT NULL, -- active, past_due, canceled, trialing
  current_period_start TIMESTAMPTZ,
  current_period_end TIMESTAMPTZ,
  cancel_at_period_end BOOLEAN DEFAULT false,
  last_invoice_paid_at TIMESTAMPTZ,
  failed_payment_count INT DEFAULT 0,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE stripe_events_processed (
  stripe_event_id VARCHAR(64) PRIMARY KEY,
  event_type VARCHAR(100) NOT NULL,
  processed_at TIMESTAMPTZ DEFAULT NOW()
);
```

## Webhook events handled

| event | acción |
|-------|--------|
| checkout.session.completed | create subscription + activar plan |
| invoice.paid | update last_invoice_paid_at + reset usage counters |
| invoice.payment_failed | failed_count++; si =3, downgrade Free |
| customer.subscription.updated | sync status/period |
| customer.subscription.deleted | downgrade Free |

## TDD plan

1. Checkout flow con stripe-mock
2. Webhook signature válida → handler corre
3. Webhook signature inválida → 401
4. Idempotency: mismo event_id 2x → 200 sin double-apply
5. 3 failed payments → downgrade
6. Cancel at period end → downgrade tras fecha (clock manipulation en test)
