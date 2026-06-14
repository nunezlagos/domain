# Design: issue-32.2-cors-allowlist-configurable

## Contexto

El dashboard (proyecto futuro, otra SPA) está en un dominio
diferente del API (`app.tudominio.com` vs `api.tudominio.com`). Los
browsers bloquean cross-origin requests por default (same-origin
policy). CORS es el mecanismo estándar para opt-in.

Hoy el server tiene CORS deshabilitado (clientes son CLIs y MCP,
todos server-to-server sin browser). Necesitamos habilitar CORS
con allowlist estricto: solo origins explícitamente autorizados.

## Decisión arquitectónica

**Estrategia:** middleware CORS configurable por env var, default
deny.

1. **Config:** `DOMAIN_CORS_ORIGINS` (CSV de origins, e.g.
   `https://app.tudominio.com,https://staging.tudominio.com`).
   Sin env var = sin CORS (default deny).

2. **Implementación:** usar `github.com/rs/cors` (librería
   standard, ~3KB, sin deps). Setup:
   ```go
   import "github.com/rs/cors"

   func NewCORSMiddleware(origins []string) *cors.Cors {
     return cors.New(cors.Options{
       AllowedOrigins: origins,
       AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
       AllowedHeaders: []string{"Authorization", "Content-Type", "X-CSRF-Token"},
       ExposedHeaders: []string{"X-Request-ID"},
       AllowCredentials: true,
       MaxAge: 86400,  // 24h
       Debug: false,
     })
   }
   ```

3. **Wildcard mode:** si `origins == ["*"]` (solo uno, exacto), el
   middleware usa `AllowedOrigins: []string{"*"}` y
   `AllowCredentials: false` (los browsers no aceptan * +
   credentials). Loggea WARNING al boot.

4. **Vary: Origin:** la librería `rs/cors` ya lo agrega
   automáticamente cuando hay múltiples origins. Verificar con
   un test.

5. **Logging de denegados:** wrapper alrededor del middleware que
   loggea con `slog.Warn("CORS denied", "origin", r.Header.Get("Origin"),
   "method", r.Method, "path", r.URL.Path)`. Útil para detectar
   ataques o configs faltantes.

6. **Aplicación:** wrappear SOLO `/api/v1/*` con CORS, NO `/health`
   ni `/api/v1/openapi.json` (esos son públicos y no necesitan
   CORS — el browser puede leerlos sin preflight si no es AJAX,
   pero si lo son, sin CORS headers está OK porque no llevan
   credentials).

7. **Manejo de Origin vacío:** si `Origin` header está ausente
   (e.g. request de server-to-server, curl), el middleware
   `rs/cors` lo trata como same-origin → no agrega CORS
   headers. Esto es deseable: no rompe el flow API key.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | CORS siempre abierto (`*`) | Inseguro. Cualquiera puede llamar al API desde un browser malicioso. |
| B | CORS basado en regex match (e.g. `*.tudominio.com`) | Más flexible pero más propenso a errores (`.com` matchea `evilcom`). Allowlist explícito es más seguro. |
| C | Custom middleware escrito a mano | `rs/cors` ya maneja preflight, credentials, vary, edge cases. Reinventar es bug-prone. |
| D | SameSite=None para cookies cross-site | Problema: Chrome depreca cookies third-party. CORS + credentials sigue siendo el path. |

## Por qué allowlist + rs/cors gana

- **Seguro:** default deny. Cero riesgo de CORS abierto
  accidental.
- **Estándar:** `rs/cors` es la librería más usada en Go. Cumple
  spec. Edge cases manejados.
- **Observable:** loggea denials, fácil de detectar configs
  faltantes.
- **Compatible:** server-to-server (sin Origin) sigue
  funcionando sin tocar nada.

## Detalle de implementación

- `internal/api/middleware/cors.go`:
  ```go
  func NewCORS(origins []string) *cors.Cors {
    if len(origins) == 0 {
      return cors.New(cors.Options{
        // allow all origins (default behavior when no allowlist)
        // pero no usamos — retornamos un middleware que no hace
        // nada para CORS
      })
    }
    if len(origins) == 1 && origins[0] == "*" {
      slog.Warn("CORS wildcard enabled; NOT for production")
      return cors.New(cors.Options{
        AllowedOrigins: []string{"*"},
        AllowedMethods: []string{"GET","POST","PATCH","DELETE","OPTIONS"},
        AllowedHeaders: []string{"Authorization","Content-Type","X-CSRF-Token"},
        MaxAge: 86400,
      })
    }
    return cors.New(cors.Options{
      AllowedOrigins: origins,
      AllowedMethods: []string{"GET","POST","PATCH","DELETE","OPTIONS"},
      AllowedHeaders: []string{"Authorization","Content-Type","X-CSRF-Token"},
      ExposedHeaders: []string{"X-Request-ID"},
      AllowCredentials: true,
      MaxAge: 86400,
    })
  }
  ```

- `cmd/domain/main.go`: leer `DOMAIN_CORS_ORIGINS` de env (CSV
  split). Si vacío, no wrappear con CORS. Si tiene valores,
  wrappear `/api/v1/*` con el middleware.

- Test: `TestCORSMiddleware_AllowsListedOrigin`, etc.

## Riesgos

- **R1:** Cambio de env var requiere restart. **Aceptable:** es
  ops normal, documentado en deploy.
- **R2:** Si el dashboard se mueve a otro dominio y olvidan
  actualizar la env var, todo falla. **Mitigación:** el log de
  denials es explícito; el dashboard debe mostrar errores CORS
  visibles al dev.
- **R3:** Wildcard `*` puede colarse a prod. **Mitigación:**
  WARNING loud al boot + README.
