# issue-31.4-client-config-remote-url

**Origen:** `REQ-31-mcp-http-vps-mode`
**Prioridad tentativa:** alta
**Tipo:** feature (CLI)

## Historia de usuario

**Como** developer configurando domain-mcp para apuntar al VPS (en lugar de a un Postgres local)
**Quiero** un flag `--remote-url https://api.tudominio.com` en `domain install` que configure el modo remoto
**Para** no tener que editar manualmente `~/.config/domain/env` ni recordar el nombre exacto de la env var

## Criterios de aceptación

### Escenario 1: `--remote-url` configura modo remoto

```gherkin
Dado que corro `domain install --remote-url https://api.tudominio.com --non-interactive`
Cuando el install procesa el flag
Entonces escribe `DOMAIN_REMOTE_URL=https://api.tudominio.com` en `~/.config/domain/env`
Y remueve (o comenta) `DOMAIN_DATABASE_URL` del mismo archivo (no debe quedar ambos en modo remoto)
Y setea `DOMAIN_BASE_URL` igual al remote-url (para que la CLI `domain projects ls` también funcione remoto)
Y loggea: "configured remote mode; pointing to https://api.tudominio.com"
```

### Escenario 2: Skip de pasos que solo aplican a server

```gherkin
Dado que el install se corre con `--remote-url`
Cuando llega a los steps de "infra local"
Entonces SKIP:
  - Step 4: "Starting services (local)" — no docker compose up
  - Step 5: "Applying migrations" — el server remoto ya las tiene aplicadas
  - Step 6: "Running seeders" — idem
Y CONTINÚA con:
  - Step 7: Global MCP env (pero solo escribe DOMAIN_REMOTE_URL, no DSN)
  - Step 8: Systemd service — SKIP (el server es remoto, no hay local)
  - Step 9: API key — prompt o usa la ya provista via --api-key
  - Step 10: Configure agents (opencode, claude-code) — ejecuta
  - Step 11: Init — SKIP (el server remoto ya tiene los .md importados, o el user puede hacerlo manual)
Y summary: "remote mode install (no local infra)"
```

### Escenario 3: `--api-key` permite no-interactive completo

```gherkin
Dado que el install corre con `--remote-url https://api.tudominio.com --api-key sk_xxx --non-interactive`
Cuando llega al step "API key"
Entonces NO pide email, NO crea first-run account
Y guarda la key en `~/.config/domain/credentials.json` (con el resto del flujo)
Y no requiere que el server sea accesible localmente (la validación de la key es online via GET /api/v1/auth/first-run)
```

### Escenario 4: Validación de scheme

```gherkin
Dado que paso `--remote-url http://api.tudominio.com` (HTTP, no HTTPS)
Cuando corro el install
Entonces el install ABORTA con: "remote-url must use https:// (got http://). Use https except for http://localhost in dev"
Y exit code 2
```

### Escenario 5: Idempotencia — install --remote-url 2 veces

```gherkin
Dado que ya tengo `~/.config/domain/env` con `DOMAIN_REMOTE_URL=https://api.tudominio.com`
Cuando corro `domain install --remote-url https://api.tudominio.com --non-interactive` de nuevo
Entonces el install detecta que ya está configurado y:
  - Re-confirma el valor (no cambia nada si coincide)
  - Skip los steps que ya se hicieron
  - Imprime: "already configured for https://api.tudominio.com (skip)"
```

### Escenario 6: Sabotaje — `DOMAIN_DATABASE_URL` queda también seteada

```gherkin
Dado que el install --remote-url fue correcto
Y el código tiene un bug (sabotaje) que NO remueve `DOMAIN_DATABASE_URL` del .env
Cuando arranco `domain-mcp`
Entonces el binario está en modo AMBIGUO (ambas env vars seteadas)
Y el test e2e que assserta "DOMAIN_DATABASE_URL ausente en modo remoto" DEBE FALLAR
Cuando restauro la lógica que UPSERT quita DSN
Entonces el test verde
```

### Escenario 7: Edge case — `--remote-url` con path

```gherkin
Dado que paso `--remote-url https://api.tudominio.com/v1` (con path)
Cuando el install configura el env
Entonces normaliza: quita el path (los endpoints se construyen relativos)
Y guarda `DOMAIN_REMOTE_URL=https://api.tudominio.com`
```

## Notas

- El install en modo remoto es estrictamente diferente del local:
  menos pasos, ningún side effect local (no docker, no migrate, no
  seed). Documentado en `cmd/domain/install_cli.go`.
- El flag `--api-key` permite configurar un cliente para un server
  YA EXISTENTE sin pasar por el flujo first-run (que requiere
  crear org+user en el server, lo cual es responsabilidad del
  operador del server, no del cliente).
- La validación online de la key (`GET /api/v1/auth/first-run` con
  Bearer) puede fallar por red (firewall, DNS temporal). El
  install no debe abortar — guarda la key y deja que el user
  diagnostique con `domain projects ls`.
