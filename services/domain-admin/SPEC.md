# Domain Admin — Refactoring SPEC

## Contexto

Domain Admin es una app Django que actúa como panel de administración para el sistema domain. La arquitectura actual funciona pero viola principios SOLID en varias áreas críticas. Este spec documenta las 5 fases de refactoring para lograr código mantenible y testeable.

## Stack

- **Framework**: Django 5.x
- **Python**: 3.12+
- **DB**: PostgreSQL 16 con pgcrypto
- **Auth**: Session-based con decorators

## Fases

---

### Fase 1: Refactor `apikeys/services.py` — SQL Crudo → Abstracción

**Problema**: Líneas 176-181 y 196-218 usan `connection.cursor()` directo con SQL:
```python
cursor.execute("UPDATE auth_api_keys SET key_ciphertext = pgp_sym_encrypt(%s, %s) ...")
```

**Solución**:
- Crear clase `ApiKeyRepository` que abstrae el acceso a DB
- Usar Django ORM para todas las operaciones
- Mantener `pgcrypto` para cifrado pero detrás de la abstracción
- La función `get_api_key_plaintext` se convierte en método del repository

**Archivos a tocar**:
- `app/maintainers/apikeys/services.py` — refactorizar
- `app/maintainers/apikeys/repository.py` — CREAR nueva abstracción

**Criterio de done**: Tests pasan, misma funcionalidad, sin `connection.cursor()`

---

### Fase 2: Refactor `users/views.py` — DIP (Dependency Inversion Principle)

**Problema**: Views de users importan directo de apikeys:
```python
from maintainers.apikeys.models import ApiKey
from maintainers.apikeys.services import get_api_key_plaintext, list_api_keys
```

**Solución**:
- Crear interfaz `ApiKeyServiceInterface` (abstracta)
- Inyectar la dependencia en `UsersView` via constructor o method injection
- Mantener `ApiKeyService` implementando la interfaz
- Usar Django's dependency injection o pasar como argumento

**Archivos a tocar**:
- `app/maintainers/users/views.py` — usar abstracción inyectada
- `app/maintainers/apikeys/services.py` — implementar interfaz explícita
- `app/maintainers/apikeys/interfaces.py` — CREAR

**Criterio de done**: Users views no importan modelos concretos de apikeys

---

### Fase 3: Split `projects/services.py` — God Class → Servicios Cohesivos

**Problema**: 473 líneas mezclando:
- CRUD proyectos
- Gestión repositorios git (`_sync_repositories`)
- Gestión skills (`list_project_skills`, `set_skill_excluded`)
- Gestión policies (`list_project_rules`, `toggle_project_policy`)
- Stats y export

**Solución**: SPLIT en 4 servicios:

1. **`ProjectService`** — solo CRUD de proyectos
   - `list_projects`, `create_project`, `update_project`, `delete_project`, `toggle_project_status`

2. **`ProjectRepositoryService`** — solo repositorios git
   - `_sync_repositories` (interno)

3. **`ProjectSkillService`** — solo skills por proyecto
   - `list_project_skills`, `set_skill_excluded`

4. **`ProjectPolicyService`** — solo policies por proyecto
   - `list_project_rules`, `toggle_project_policy`

**Interfaces** para comunicación inter-servicios:
- `ProjectService` necesita `ProjectRepositoryService`, `ProjectSkillService`, `ProjectPolicyService`
- Usar composición: `ProjectService` recibe otros servicios inyectados

**Archivos a tocar**:
- `app/maintainers/projects/services.py` — SPLIT
- `app/maintainers/projects/project_service.py` — CREAR
- `app/maintainers/projects/project_repository_service.py` — CREAR
- `app/maintainers/projects/project_skill_service.py` — CREAR
- `app/maintainers/projects/project_policy_service.py` — CREAR
- `app/maintainers/projects/interfaces.py` — CREAR abstracciones

**Criterio de done**: Ningún servicio importa modelos de otro dominio directamente

---

### Fase 4: Refactor `core/views.py` — SRP (Single Responsibility Principle)

**Problema**: 341 líneas con múltiples responsabilidades:
- Auth checking (`require_auth` wrapper)
- AJAX detection (`is_ajax`)
- Context building (4 métodos distintos)
- HTTP response handling
- Service discovery por convención

**Solución**: Dividir en 3 mixins + 1 clase liviana

1. **`AuthMixin`** — solo autenticación
   - Decorator `require_auth` aplicado a métodos

2. **`AjaxMixin`** — solo detección AJAX
   - `is_ajax()` y helper `render_ajax()`

3. **`ContextMixin`** — solo context building
   - `_get_list_context()`, `_get_detail_context()`, etc.

4. **`MaintainerView`** — solo orchestration
   - Usa los mixins
   - Mantiene el hook pattern (`do_list`, `do_create`, etc.)
   - Discovery de servicios por convención (OK por ahora)

**Archivos a tocar**:
- `app/core/views.py` — refactorizar con mixins
- `app/core/mixins.py` — CREAR con AuthMixin, AjaxMixin, ContextMixin

**Criterio de done**: Ninguna clase/función > 150 líneas en core/

---

### Fase 5: Eliminar Comentarios Innecesarios

**Regla**: Solo mantener comentarios que expliquen POR QUÉ, no QUÉ.

**Eliminar**:
- Docstrings redundantes que repiten el nombre de la función
- Comentarios tipo "This function does X" cuando el código ya dice eso
- Blanks informativos entre secciones (///, ###, etc.)
- TODO comments (mover a issues tracker)

**Conservar**:
- Decisiones de diseño no obvias ("We use X because Y")
- Workarounds con referencia a bug/external constraint
- Explicaciones de lógica compleja que no se puede inferir del código

**Archivos a tocar**: TODOS los `.py` del proyecto

**Criterio de done**: `grep -r "# " --include="*.py" | wc -l` reduce significativamente

---

## Métricas de Éxito

| Fase | Métrica |
|------|---------|
| 1 | 0 `connection.cursor()` en apikeys |
| 2 | 0 imports de modelos concretos entre dominios en views |
| 3 | Cada service < 200 líneas |
| 4 | `core/views.py` < 150 líneas |
| 5 | Comentarios tipo QUÉ eliminados |

## Orden de Implementación

1. ✅ Spec creado
2. 🔄 Fase 1: apikeys/services.py
3. ⬜ Fase 2: users/views.py + apikeys interfaces
4. ⬜ Fase 3: projects/services.py split
5. ⬜ Fase 4: core/views.py split
6. ⬜ Fase 5: cleanup comentarios
7. ⬜ Tests y verificación final
