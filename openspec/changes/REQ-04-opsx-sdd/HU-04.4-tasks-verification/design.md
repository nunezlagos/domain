# Design: HU-04.4-tasks-verification

## Decisión arquitectónica

**3 tablas (tasks 1:N verification_results, 1:N sabotage_records). Status machine con transiciones estrictas.**

```
tasks
├── id              UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── hu_id           UUID NOT NULL REFERENCES user_stories(id) ON DELETE CASCADE
├── section         VARCHAR(50) NOT NULL           -- Backend | Tests | Cierre
├── description     TEXT NOT NULL
├── status          VARCHAR(20) NOT NULL DEFAULT 'pending'  -- pending | in_progress | completed
├── position        INT NOT NULL DEFAULT 0
├── started_at      TIMESTAMPTZ
├── completed_at    TIMESTAMPTZ
├── completed_by    VARCHAR(255)
├── created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()

verification_results
├── id              UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── task_id         UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE
├── result          VARCHAR(20) NOT NULL            -- pass | fail | blocked
├── evidence        TEXT
├── notes           TEXT
├── verified_at     TIMESTAMPTZ NOT NULL DEFAULT now()
└── verified_by     VARCHAR(255)

sabotage_records
├── id              UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── task_id         UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE
├── action          TEXT NOT NULL                   -- qué se rompió
├── expected_failure TEXT                           -- qué se esperaba
├── actual_result   TEXT                            -- qué pasó realmente
├── restored        BOOLEAN NOT NULL DEFAULT true
└── performed_at    TIMESTAMPTZ NOT NULL DEFAULT now()
```

**Status machine:**
```
pending ──→ in_progress ──→ completed
  │                            │
  └──── (no retroceso) ───────┘
```

**Progress query:**
```sql
SELECT
  hu_id,
  COUNT(*) AS total,
  COUNT(*) FILTER (WHERE status = 'completed') AS completed,
  ROUND(100.0 * COUNT(*) FILTER (WHERE status = 'completed') / COUNT(*), 1) AS progress_pct
FROM tasks
WHERE hu_id = $1
GROUP BY hu_id;
```

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| Tasks como JSONB array en HU | Dificulta queries individuales y joins con verificación |
| Verification embebida en task | Normalización: 1 registro de verificación puede tener múltiples evidencias |
| Sabotaje como tipo especial de task | Es un registro aparte; una task puede tener múltiples sabotajes |
| Status con bitmask | Menos legible; VARCHAR con validación es más claro |

## Diagrama

```
user_stories (1) ──→ (N) tasks (1) ──→ (N) verification_results
                      (1) ──→ (N) sabotage_records

Task lifecycle:
  [pending] → set started_at → [in_progress] → set completed_at → [completed]
                                                      │
                                                      ▼
                                            verification_results
                                              └── pass / fail / blocked

  [task of type sabotage]
    └── sabotage_records
          └── action, expected_failure, actual_result, restored
```

## TDD plan

1. **Red**: Test: crear tareas batch → listar por HU → todas pending
2. **Green**: Implementar CreateTasks + ListTasks
3. **Red**: Test: status transition pending → in_progress → completed
4. **Green**: Implementar UpdateTaskStatus con validación
5. **Red**: Test: completar 6/10 → progress = 60%
6. **Green**: Implementar GetProgress
7. **Red**: Test: registrar verificación en tarea completada
8. **Green**: Implementar CreateVerification
9. **Red**: Test: registrar sabotaje
10. **Green**: Implementar CreateSabotage
11. **Sabotaje**: verificar tarea pending → error

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Status saltado (pending → completed) | Validación en service: solo transiciones 1 paso |
| Verificación sin tarea completada | Validar task.status = completed |
| Sabotaje restaurado = false | Notificación en reports |
