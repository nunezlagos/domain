# Proposal: issue-38.11-readme-update

## Intención

Reescribir el `README.md` de la rama services para reflejar la arquitectura
multi-servicio (5 containers + Caddy), el flujo de versionado de imágenes
GHCR, y la operación día-a-día con `make`, manteniendo el principio de
brevedad (<100 líneas).

## Scope

**Incluye:**
- Reescribir las secciones:
  - **Header**: 1-2 líneas explicando qué hace la rama services.
  - **Diagrama**: ASCII compacto mostrando los 5 containers + Caddy + flujo
    de tráfico HTTP.
  - **Instalación**: 3 comandos para deploy en VPS.
  - **Layout**: árbol compacto de las carpetas + 1 línea descriptiva c/u.
  - **Operación**: comandos `make` actualizados.
  - **Acceso**: URLs finales `http://<vps-ip>/...`.
  - **Backups**: cómo restaurar + cuándo corre + retención.
  - **Update**: flujo `git tag → CI → .env → make pull && restart`.

**No incluye:**
- Sección FAQ extensa (decisión: README corto).
- Troubleshooting de 10 escenarios (vive en docs/ del backend si hace falta).
- Documentación del cliente MCP (eso es de REQ-CLIENT, otro repo/lugar).
- Mención de dominio o TLS (decisión cerrada: HTTP plano por IP).

## Enfoque técnico

1. **Mantener tono actual**: el README actual es directo, sin emojis,
   profesional. Conservar.
2. **Diagrama ASCII compacto** (sin grafiti, sin colores):
   ```
   INTERNET → Caddy :80 ─┬─ /api/* /mcp* /healthz → domain-backend:8000
                         └─ /*                     → domain-frontend:80
                                                          │
                              red interna domain_internal:│
                                  postgres ─── minio ─────┘
   ```
3. **Tablas para acceso/operación**: más fáciles de escanear que prosa.
4. **Comandos en bloques**: lectura visual rápida.
5. **Sin "Decisiones de diseño" / FAQ**: el contenido reflexivo vive en
   openspec/. README es operativo.

## Riesgos

- **Información obsoleta vs realidad**: si los comandos cambian (HU-38.8)
  y el README no se actualiza, divergencia. Mitigación: esta HU es la última
  de la wave 5, ya con HU-38.8 y HU-38.10 completas.
- **Demasiado terso**: si el operador no entiende qué hace cada cosa, busca
  en otro lado. Mitigación: links a docs/ del backend para contenido extenso.

## Testing

- `wc -l README.md` <= 100 líneas
- Manual: leer README de arriba abajo en <2 min, entender qué deployar y cómo
- Todos los comandos `make` mencionados existen y funcionan (cross-check con
  Makefile post HU-38.8)
- Todas las URLs mencionadas resuelven correctamente con el stack arriba
- Cero menciones de dominio, TLS, https, sslip, cloudflare
- Cero secretos o ejemplos con credenciales reales
- Diagrama renderiza bien en GitHub (probar con preview)
