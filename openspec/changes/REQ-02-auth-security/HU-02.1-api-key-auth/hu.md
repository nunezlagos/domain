# HU-02.1-api-key-auth

**Origen:** `REQ-02-auth-security`
**Persona:** platform-engineer, security-officer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** administrador de la plataforma
**Quiero** generar API keys seguras (32+ bytes aleatorios con prefijo identificador), almacenarlas hasheadas con bcrypt, y gestionarlas (CRUD, rotación, revocación)
**Para** que los integrantes de mi organización puedan autenticarse de forma segura ante la API de Domain

## Criterios de aceptación

### Escenario 1: Generación de API key con prefijo

```gherkin
Dado que soy un administrador autenticado
Cuando solicito generar una nueva API key con nombre "prod-deploy-key"
Entonces recibo una key en formato `mem_<random_base64>`
Y el prefijo `mem_` está presente
Y la key completa tiene al menos 40 caracteres
Y la key almacenada en DB es el hash bcrypt de la key original
Y el key_prefix almacenado son los primeros 8 caracteres de la key original
Y la respuesta incluye la key original (una sola vez)
```

### Escenario 2: Autenticación con API key válida

```gherkin
Dado que existe una API key activa no expirada y no revocada
Cuando hago una petición a un endpoint protegido con header `X-API-Key: <key_original>`
Entonces la autenticación es exitosa
Y el request context contiene el organization_id y user_id asociados
```

### Escenario 3: Autenticación con API key inválida

```gherkin
Dado que la API key no existe o no coincide con ningún hash
Cuando hago una petición a un endpoint protegido con header `X-API-Key: mem_invalidkey123`
Entonces recibo `401 Unauthorized`
Y el body indica "invalid or revoked API key"
```

### Escenario 4: API key expirada

```gherkin
Dado que una API key tiene `expires_at` en el pasado
Cuando hago una petición a un endpoint protegido con esa key
Entonces recibo `401 Unauthorized`
Y el body indica "API key expired"
```

### Escenario 5: API key revocada

```gherkin
Dado que una API key tiene `revoked_at` no nulo
Cuando hago una petición a un endpoint protegido con esa key
Entonces recibo `401 Unauthorized`
Y el body indica "API key revoked"
```

### Escenario 6: Rotación de API key

```gherkin
Dado que existe una API key activa con id X
Cuando solicito rotar la key X
Entonces se genera una nueva key (hash + prefijo distintos)
Y la key anterior se marca como revocada
Y recibo la nueva key original (una sola vez)
```

### Escenario 7: Listado y borrado de keys

```gherkin
Dado que existen 3 API keys en mi organización
Cuando solicito listar las keys
Entonces recibo un array con 3 elementos
Y cada elemento contiene: id, prefix, name, created_at, expires_at, revoked_at
Y NUNCA incluye el key_hash ni la key original

Cuando solicito borrar una key
Entonces la key se elimina físicamente de la DB
Y ya no aparece en el listado
```

## Análisis breve

- **Qué pide realmente:** Sistema de API keys con generación segura, hashing bcrypt, middleware de validación, operaciones CRUD, rotación atómica, expiración y revocación.
- **Módulos sospechados:** `internal/auth/`, `internal/api/middleware/`, `internal/models/`
- **Riesgos / dependencias:** La key original solo se muestra una vez en generación/rotación. El hash bcrypt es costoso (configurable). La key debe ser aleatoria criptográfica, no pseudoaleatoria.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
