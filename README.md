# domain-services

Plataforma Domain: backend MCP, observability, SDD, installer.

## VPS Quickstart

Pegá estos comandos en la sesion SSH del VPS (`ssh sysadmin@13.140.183.236`).
Los scripts son idempotentes: reinstalar o redeploy no rompe nada.

### 1) Instalar el runner (una sola vez por VPS)

```bash
cd /opt/services && \
  git fetch origin && \
  git checkout main && \
  git reset --hard origin/main && \
  ./scripts/install-runner.sh --all
```

Cuando lo pegues, el script te pide el **registration token** (no se ve al
tipear). Lo generas una vez desde GitHub:
**repo -> Settings -> Actions -> Runners -> New self-hosted runner** -> copia
el token de la linea `./config.sh --token ...`.

### 2) Redeploy manual (cada vez que quieras)

```bash
cd /opt/services && ./scripts/redeploy.sh
```

Hace lo mismo que el CI en push a main: pull + build + restart + verify, con
rollback automatico si algo falla.

Para ver que haria sin tocar nada: `./scripts/redeploy.sh --dry-run`.

## Documentacion

- `INSTALL.md` -- guia del instalador `domain install` (cliente local).
- `AGENTS.md` -- reglas para agentes IA que trabajan en este repo.
- `openspec/changes/` -- issues SDD con Gherkin (source of truth de specs).