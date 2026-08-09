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

### 3) Deploy automatico: publicar un tag alcanza

Desde `v0.7.3` (issue-57.2) el VPS se despliega solo. `domain-auto-deploy.timer`
revisa cada 10 minutos y despliega **solo el ultimo tag `v*` que ademas sea la
punta de `origin/main`**. Un push a main sin tag no despliega nada: eso es
deliberado, para que no se publique codigo sin version.

```bash
git push origin main
git push origin v1.2.0   # `git push` NO empuja tags: va explicito
```

Esto revierte de hecho la regla previa de "CI si, CD no" del 2026-07-27, por
decision explicita del usuario.

Hicieron falta tres arreglos y ninguno se notaba desde afuera, porque el
mecanismo fallaba **en silencio y siempre**:

- el unit no declaraba `HOME`, y sin `HOME` git no lee el `safe.directory` de
  `/root/.gitconfig`: exit 128 en cada corrida desde que el timer existe
- el validate del `.env` medía con `-w`, que bajo root es siempre verdadero
  porque root ignora los bits de permiso: 61 corridas, 61 abortos, cero deploys
- `log_phase` escribia con `tee`, asi que un `.deploy.log` no escribible mataba
  el deploy en su primera linea — y de paso dejaba caido el redeploy manual

Si un ciclo falla, ahora avisa por ntfy en vez de morir callado.

Para saber si un tag llego al VPS, mirar el HEAD del VPS y no el hecho de
haberlo publicado:

```bash
ssh sysadmin@13.140.183.236 'git -C /opt/services describe --tags'
```

#### Cuando el arreglo del deploy no puede llegar por el deploy

`deploy.sh` valida el entorno ANTES de hacer el checkout, asi que un bug en esa
validacion se blinda a si mismo: el codigo del VPS aborta antes de traer el
codigo que lo corrige. Paso dos veces con issue-57.2.

Para romperlo hace falta UNA corrida manual que evada la validacion rota. No se
parchea el script a mano: `DEPLOY_ENV_FILE` ya existe para esto.

```bash
# el .env real, no services/.env que es un symlink hacia el
cd /opt/services && PREV_SHA=$(git rev-parse HEAD) \
  DEPLOY_ENV_FILE=/opt/services/.env ./scripts/redeploy.sh
```

`PREV_SHA` es obligatorio: sin el, `deploy.sh` aborta en fetch. Y va el SHA
ACTUAL del VPS, tomado antes del reset, porque es el punto desde el que se
detectan los cambios.

Si la validacion rota no se puede evadir con una variable, la salida es
`sudo bash services/install.sh`, que hace fetch y reset por su cuenta sin pasar
por `deploy.sh`.

## Documentacion

- `INSTALL.md` -- guia del instalador `domain install` (cliente local).
- `AGENTS.md` -- reglas para agentes IA que trabajan en este repo.
- `openspec/changes/` -- issues SDD con Gherkin (source of truth de specs).