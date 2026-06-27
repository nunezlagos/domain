# INSTALL — domain-services

Guía de instalación, reinstalación y operación del instalador `domain install`.

El objetivo central del instalador es la **IDEMPOTENCIA**: reinstalar nunca
pierde datos ni rompe la configuración existente. Migraciones y seeders son
idempotentes, los secretos generables se persisten y reúsan, y toda config
externa (.env, credentials.json, opencode.json, settings.json) se respalda
antes de tocarse.

---

## 1. Instalación fresca (paso a paso)

### Requisitos

- Linux / macOS / WSL2 (Windows nativo: `bootstrap.ps1`).
- `curl` + `tar` (presentes en cualquier distro base).
- Go >= 1.22 **opcional**: si no está, el bootstrap baja uno local a
  `~/.local/go/` (sin sudo, sin tocar `/usr`).
- Docker + docker-compose (modo `local` levanta Postgres/infra vía compose).

### Pasos

1. Clonar el repo y posicionarse en la raíz (`domain-services/`).
2. Ejecutar el bootstrap:

   ```bash
   ./install-user/bootstrap.sh
   ```

   - Detecta/instala Go, compila `domain-install`, y ejecuta `domain install`
     pasándole los argumentos recibidos.
   - El binario queda en `install-user/domain-install`; re-ejecuciones saltean
     Go install + build si ya existe y es reciente.

3. Modo no interactivo (CI / scripted):

   ```bash
   ./install-user/bootstrap.sh \
     --mode local \
     --email admin@example.com \
     --non-interactive
   ```

### Qué hace `domain install` (pasos del CLI)

1. **Detecting state** — detecta si ya hay una instalación previa.
2. **Backing up configs** — respalda `.env`, `credentials.json`,
   `opencode.json`, `settings.json` antes de mutar (omitible con `--no-backup`).
3. **Bootstrap .env** — genera/preserva `.env` desde `services/.env.example`.
4. **Starting services** — levanta infra (docker-compose) según `--mode`.
5. **Applying migrations** — idempotentes.
6. **Running seeders** — idempotentes.
7. **Field encryption key** — genera/reúsa/valida `DOMAIN_FIELD_ENC_KEY`
   (ver sección 4).
8. **Global MCP env** — escribe `~/.config/domain/env`.
9. **Feature flags (opt-in)** — persiste los `DOMAIN_*_ENABLED=true` activados
   (ver sección 3).
10. **Starting server (systemd)** — servicio de usuario (omitible con
    `--no-service`).
11. **API key** — first-run crea la cuenta; reinstall emite nueva API key admin.
12. **Configuring MCP agents** — registra el MCP `domain-mcp` en OpenCode y
    Claude Code (idempotente).
13. **Shell wrapper / hooks / global instructions / primary-memory / import .md**
    — pasos opcionales según flags.

---

## 2. Reinstalación (qué se preserva, qué NO se pierde)

Reinstalar = volver a correr `./install-user/bootstrap.sh`. NADA se pierde:

| Activo | Comportamiento en reinstall |
|---|---|
| **Migraciones** | Idempotentes — no re-aplican lo ya aplicado. |
| **Seeders** | Idempotentes — upsert, sin duplicar. |
| **`DOMAIN_FIELD_ENC_KEY`** | Se **reúsa** la persistida; nunca se regenera. Si las keys cifradas existentes no se pueden descifrar con la key actual, el paso **ABORTA** (no pisa). |
| **Feature flags** | Lo persistido en `~/.config/domain/env` se **preserva**; reinstalar sin `--enable-X` no desactiva nada. |
| **`.env` / `credentials.json` / `opencode.json` / `settings.json`** | Se **respaldan** antes de tocar (`.bak.<timestamp>`); config previa se conserva. |
| **Registro MCP** | Idempotente: no duplica entradas; preserva otras. |
| **API keys emitidas** | Intactas; reinstall solo emite una nueva key admin. |

> Restaurar un backup puntual: `domain restore ~/.config/domain/credentials.json.bak.<timestamp>`

---

## 3. Matriz de features / flags (opt-in)

Las 5 features de cron son **opt-in con default seguro `false`**. El instalador
solo persiste `DOMAIN_*_ENABLED=true` para las que actives explícitamente con
`--enable-X`; sin activación, `config.go` defaultea a `false` y el cron no corre.

| Flag (`domain install ...`) | Env var persistida | Qué hace | LLM |
|---|---|---|---|
| `--enable-edge-inference` | `DOMAIN_EDGE_INFERENCE_ENABLED` | Infiere edges de relación entre observaciones | Sí |
| `--enable-feedback-aggregator` | `DOMAIN_FEEDBACK_AGGREGATOR_ENABLED` | Agrega feedback de uso de skills | No |
| `--enable-skill-metrics` | `DOMAIN_SKILL_METRICS_ENABLED` | Rollup de métricas de skills (hourly/daily/weekly) | No |
| `--enable-skill-judge` | `DOMAIN_SKILL_JUDGE_ENABLED` | Evalúa calidad de skills semanalmente (LLM-as-judge) | Sí |
| `--enable-ab-test` | `DOMAIN_AB_TEST_ENABLED` | Analiza experimentos A/B de skills | No |

### Cómo activar

```bash
# Activar un cron
./install-user/bootstrap.sh --enable-skill-metrics

# Activar varios
./install-user/bootstrap.sh --enable-edge-inference --enable-skill-judge --enable-ab-test
```

Las features con LLM (`edge-inference`, `skill-judge`) **degradan** (no corren)
si no hay `LLM_API_KEY` configurada.

### Tuning fino (env opcionales, defaults seguros)

Ajustables vía `~/.config/domain/env` o `.env` (no expuestos como flags):

- Edge inference: `DOMAIN_EDGE_INFERENCE_TICK_HOURS=6`, `_MAX_PAIRS=30`, `_PROJECT_BATCH=50`
- Feedback: `DOMAIN_FEEDBACK_AGGREGATOR_TICK_HOURS=6`, `_DAYS=7`
- Skill metrics: `DOMAIN_SKILL_METRICS_TICK_HOURS=1`, `_ROLLUP_TICK_HOURS=24`, `_DAILY_RETENTION_DAYS=90`, `_WEEKLY_RETENTION_DAYS=365`
- Skill judge: `DOMAIN_SKILL_JUDGE_WEEKDAY=1`, `_HOUR=3`, `_MAX_SKILLS=200`
- A/B test: `DOMAIN_AB_TEST_TICK_HOURS=6`, `_ALPHA=0.05`, `_AUTO_APPLY=false`

---

## 4. Variables de entorno requeridas

### `LLM_API_KEY` (opcional — externa)

- Nombre **genérico/primario** de la key del LLM (endpoint anthropic-compatible).
- Fallback de compat: `LLM_API_KEY` → `MINIMAX_API_KEY` → `DOMAIN_MINIMAX_API_KEY` (solo Go).
- Modelo: `LLM_MODEL` → `MINIMAX_MODEL` → default `MiniMax-M3`.
- **Si falta, las features LLM degradan** (no se generan); el resto funciona.
- `MINIMAX_*` queda como alias **DEPRECADO** (no romper instalaciones viejas).

### `DOMAIN_FIELD_ENC_KEY` (auto-generada — secreto del VPS)

- Clave de cifrado de las API keys (`pgp_sym_encrypt` sobre
  `auth_api_keys.key_ciphertext`). El **cliente NO la necesita**: solo
  Issue/Rotate la usan; Resolve autentica por hash.
- **El cliente no la setea.** El instalador la maneja así:
  - **Fresca**: si no está seteada ni persistida → genera una aleatoria fuerte
    (`crypto/rand`, 32 bytes → base64) y la persiste en `~/.config/domain/env`
    (y `.env`).
  - **Reinstall**: si ya hay una persistida → la **reúsa** (nunca regenera).
  - **Validación**: si hay filas con `key_ciphertext` y la enc-key actual NO
    puede descifrarlas (canary `pgp_sym_decrypt`) → **ABORTA** con error claro,
    para no perder keys.

---

## 5. Troubleshooting

**Reinstalar, ¿pierdo algo?**
No. Migraciones/seeders son idempotentes, la enc-key se reúsa, y toda config se
respalda antes de tocarse. Ver tabla en sección 2.

**El paso "Field encryption key" ABORTA.**
La enc-key actual no descifra las keys ya emitidas. Significa que la
`DOMAIN_FIELD_ENC_KEY` cambió respecto a la usada al emitirlas. Restaurá la key
original en `~/.config/domain/env` y reintentá. NO la regeneres a mano: pisarla
inutiliza las keys cifradas.

**Las features LLM (edge-inference / skill-judge) no corren.**
Falta `LLM_API_KEY`. Seteala en `~/.config/domain/env` (o `.env`) y reiniciá el
servicio. Sin ella degradan silenciosamente.

**Activé un flag pero el cron no arranca.**
Verificá que `DOMAIN_*_ENABLED=true` quedó en `~/.config/domain/env` y reiniciá
el servidor (`systemctl --user restart domain` o relanzá si usaste `--no-service`).

**Quiero desactivar una feature.**
Editá `~/.config/domain/env` y poné `DOMAIN_*_ENABLED=false` (o borrá la línea;
el default es `false`). Reiniciá el servidor.

**Go no detectado / versión vieja.**
El bootstrap baja Go local a `~/.local/go/` automáticamente. No requiere sudo.

**Restaurar una config previa.**
Los backups quedan junto al archivo original como `.bak.<timestamp>`. Ej:
`domain restore ~/.config/domain/credentials.json.bak.<timestamp>`.
