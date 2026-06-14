# Design: issue-04.3-specs-designs

## Decisión arquitectónica

**Versionado por INSERT (append-only) en lugar de UPDATE. Cada versión es un registro completo. "Última versión" = MAX(version) GROUP BY issue_id.**

```
proposals
├── id                UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── issue_id             UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE
├── version           INT NOT NULL DEFAULT 1
├── status            VARCHAR(20) NOT NULL DEFAULT 'draft'  -- draft | approved | rejected
├── intention         TEXT NOT NULL
├── scope             TEXT NOT NULL
├── approach          TEXT NOT NULL          -- markdown
├── risks             TEXT                   -- markdown
├── testing_notes     TEXT                   -- markdown
├── rejection_reason  TEXT                   -- nullable, solo si rejected
├── created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()

UNIQUE(issue_id, version)

designs
├── id                UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── issue_id             UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE
├── proposal_id       UUID REFERENCES proposals(id) ON DELETE SET NULL
├── version           INT NOT NULL DEFAULT 1
├── status            VARCHAR(20) NOT NULL DEFAULT 'draft'  -- draft | final
├── arch_decisions    TEXT NOT NULL          -- markdown
├── alternatives      TEXT                   -- markdown
├── data_flow         TEXT                   -- markdown
├── tdd_plan          TEXT                   -- markdown
├── risks_mitigation  TEXT                   -- markdown
├── created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()

UNIQUE(issue_id, version)
```

**Obtener última versión:**
```sql
SELECT * FROM proposals
WHERE issue_id = $1
ORDER BY version DESC
LIMIT 1;
```

**Crear nueva versión:**
```sql
INSERT INTO proposals (issue_id, version, status, intention, scope, approach, risks, testing_notes)
SELECT $1, COALESCE(MAX(version), 0) + 1, 'draft', $2, $3, $4, $5, $6
FROM proposals WHERE issue_id = $1;
```

**Índices:**
- `proposals_hu_id_version_idx` UNIQUE BTREE (issue_id, version)
- `proposals_status_idx` BTREE (status)
- `designs_hu_id_version_idx` UNIQUE BTREE (issue_id, version)

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| UPDATE con columna version | Se pierde historia; no hay trazabilidad de cambios |
| Tabla separada proposals_history | Complejidad innecesaria; el versionado inline es más simple |
| JSONB para todo el contenido | Se pierde la estructura de campos específicos |
| Git-based storage | Overkill; no necesitamos branching ni merging |

## Diagrama

```
issues (1) ──→ (N) proposals (1) ──→ (N) designs
     │                      │                     │
  slug, title            version, status        version, status
  req_id (FK)            intention, approach    arch_decisions, tdd_plan
                         risks (markdown)       alternatives (markdown)

Workflow:
  HU exists
    │
    ▼
  Create Proposal (v1, draft)
    │
    ├── → Approved (status change, no new version)
    │     │
    │     ▼
    │   Create Design (v1, draft, links to proposal)
    │     │
    │     └── → Final (status change)
    │
    └── → Rejected (status change + rejection_reason)
          │
          └── → New version (v2, draft) con cambios
```

## TDD plan

1. **Red**: Test: crear proposal v1 → obtener latest → v1
2. **Green**: Implementar CreateProposal + GetLatestProposal
3. **Red**: Test: crear v2 → latest es v2, v1 sigue accesible
4. **Green**: Implementar versionado por INSERT
5. **Red**: Test: status transitions válidas (draft→approved, draft→rejected)
6. **Green**: Implementar ChangeProposalStatus con validación
7. **Red**: Test: crear design vinculado a proposal
8. **Green**: Implementar DesignStore
9. **Sabotaje**: proposal rejected → crear design igual es posible (no hay bloqueo)

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Versionado incrementa espacio | Documentos pequeños (~5KB c/u); aceptable |
| Status transition inválida | Validación en service layer (no rejected → approved) |
| Design sin proposal | Permitido; proposal_id nullable |
