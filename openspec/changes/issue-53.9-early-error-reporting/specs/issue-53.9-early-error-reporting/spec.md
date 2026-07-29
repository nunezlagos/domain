# Como operator del MCP, los errores se categorizan automaticamente (SQL/AUTH/VALIDATION/TIMEOUT/PANIC/etc), se agrupan por fingerprint normalizado, disparan alertas configurables (webhook+email+ntfy) y los known-errors disparan self-healing (retry con backoff, clear cache, restart worker) sin intervencion humana.

