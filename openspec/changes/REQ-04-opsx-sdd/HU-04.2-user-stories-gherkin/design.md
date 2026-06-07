# Design: HU-04.2-user-stories-gherkin

## Decisión arquitectónica

**2 tablas (user_stories + gherkin_scenarios 1:N) + Gherkin como structured data con arrays nativos de Postgres.**

```
user_stories
├── id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── slug        VARCHAR(50) UNIQUE NOT NULL    -- "HU-01.1-db-schema"
├── title       VARCHAR(500) NOT NULL
├── description TEXT
├── status      VARCHAR(20) NOT NULL DEFAULT 'proposed'
├── priority    VARCHAR(20) NOT NULL DEFAULT 'medium'
├── req_id      UUID NOT NULL REFERENCES requirements(id) ON DELETE RESTRICT
├── created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()

gherkin_scenarios
├── id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── hu_id       UUID NOT NULL REFERENCES user_stories(id) ON DELETE CASCADE
├── feature     VARCHAR(255) NOT NULL          -- "Feature: Auth"
├── scenario    VARCHAR(500) NOT NULL          -- "Scenario: Login exitoso"
├── given       TEXT[] NOT NULL                -- {"usuario existe","contraseña válida"}
├── when        TEXT NOT NULL                  -- "login con credenciales"
├── then        TEXT[] NOT NULL                -- {"token devuelto","status 200"}
├── position    INT NOT NULL DEFAULT 0         -- orden dentro de la HU
└── created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
```

**Ejemplo de datos Gherkin:**
```
feature: "Login Feature"
scenario: "Login exitoso"
given: ["El usuario está registrado", "La cuenta está activa"]
when: "El usuario envía credenciales válidas"
then: ["El sistema devuelve un token JWT", "El status es 200"]
```

**Índices:**
- `user_stories_slug_idx` UNIQUE BTREE (slug)
- `user_stories_req_id_idx` BTREE (req_id)
- `user_stories_status_idx` BTREE (status)
- `gherkin_hu_id_idx` BTREE (hu_id)

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| Gherkin como TEXT (markdown) | No permite consultas estructuradas por paso específico |
| JSONB para escenarios | Postgres arrays son más eficientes para listas simples; JSONB agrega complejidad innecesaria |
| Escenarios como JSON en columna de user_stories | Violación de 1NF; dificulta consultas por scenario específico |
| Tabla única con escenarios embedidos | Array de JSON en Postgres es posible pero menos práctico |

## Diagrama

```
requirements (1) ──→ (N) user_stories (1) ──→ (N) gherkin_scenarios
     │                      │                        │
  slug, title            slug, title            feature, scenario
  status, priority       status, priority       given[], when, then[]
  parent_id              req_id (FK)            hu_id (FK), position

HU creation flow:
  1. Validate REQ exists
  2. Validate slug format "HU-NN.N-*"
  3. INSERT user_story
  4. INSERT gherkin_scenarios (batch)
  5. Return HU with scenarios
```

## TDD plan

1. **Red**: Test: crear HU con 2 escenarios → get → 2 escenarios
2. **Green**: Implementar Create + Get con scenarios
3. **Red**: Test: agregar escenario a HU existente
4. **Green**: Implementar AddScenario
5. **Red**: Test: filtrar HUs por req_slug
6. **Green**: Implementar List con filtros
7. **Sabotaje**: escenario sin given → validación debe fallar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Given/Then arrays muy grandes | Validar max 10 pasos por sección |
| Position duplicada | Auto-asignar con MAX(position)+1 |
| Slug mal formateado | Regex `^HU-\d+\.\d+(-[a-z0-9-]+)?$` |
