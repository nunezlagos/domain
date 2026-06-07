# HU-25.10-db-secrets-rotation

**Origen:** `REQ-25-db-tooling-hardening`
**Persona:** platform-engineer, security-officer
**Prioridad tentativa:** media
**Tipo:** hardening

## Historia de usuario

**Como** security/compliance
**Quiero** rotar password de DB sin downtime
**Para** cumplir policy de rotation 90 días sin paginar

## Modelo dual-credentials

1. Tener 2 passwords activos al mismo tiempo (`app_user` + `app_user_next`)
2. K8s Secret tiene `password_current` y `password_next`
3. Pods nuevos toman `password_next`; pods viejos siguen con `password_current`
4. Tras rollout: `password_current = password_next`, generar nuevo `password_next`

## Criterios de aceptación

### Escenario 1: Rotación scheduled

```gherkin
Dado que existe cron `db-password-rotation` cada 90 días
Cuando se ejecuta
Entonces:
  - genera nuevo password random
  - `ALTER ROLE app_user PASSWORD 'new_pwd'` (multiple passwords con role mechanism)
  - actualiza K8s Secret con field `password_next`
  - dispara rollout de Deployment app
  - pods nuevos usan password_next
  - tras 100% rollout: ALTER ROLE drop old + Secret reorganiza
Y zero downtime durante el proceso
```

### Escenario 2: External secret manager integration

```gherkin
Dado que prod usa AWS Secrets Manager (vía ESO)
Cuando rotation cron corre
Entonces actualiza el Secret en AWS Secrets Manager
Y ESO sincroniza a K8s Secret en <60s
Y rollout app pods con nuevo valor
```

### Escenario 3: Manual rotation

```gherkin
Dado que admin quiere rotar urgente (ej. password potencialmente leakeado)
Cuando ejecuta `domain-mcp rotate-db-password --role app_user`
Entonces se ejecuta misma lógica que cron
Y se logea audit "db.password.rotated_manual"
```

### Escenario 4: Rollback

```gherkin
Dado que rotation falla mid-way
Cuando se detecta
Entonces el script restaura state previo
Y app_user vuelve a password_current
Y se notifica
```

### Escenario 5: Multi-role rotation

```gherkin
Dado que existen app_user, app_admin, app_migrator, app_readonly
Cuando rotation ejecuta
Entonces rota cada uno independientemente (no atómico para todos a la vez)
Y schedule staggered (no todos el mismo día)
```

## Análisis breve

- **Qué pide:** cron rotation + ESO integration + manual command + rollback + multi-role staggered
- **Esfuerzo:** M
- **Riesgos:** rollout falla mid → apps con password viejo, otras con nuevo; coordinación con PgBouncer userlist
