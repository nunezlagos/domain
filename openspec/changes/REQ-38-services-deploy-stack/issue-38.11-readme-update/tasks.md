# Tasks: issue-38.11-readme-update

## Rewrite

- [ ] **rw-001**: Reescribir header (1-2 líneas explicando rama services).
- [ ] **rw-002**: Sumar diagrama ASCII compacto de topología.
- [ ] **rw-003**: Sección Instalación: 3 comandos + descripción 1 párrafo +
      lista de flags.
- [ ] **rw-004**: Sección Layout: tree compacto + 1 línea descriptiva por
      carpeta.
- [ ] **rw-005**: Sección Operación: bloque de comandos make.
- [ ] **rw-006**: Sección Acceso: lista de URLs http://<vps-ip>/... +
      nota sobre PG/MinIO internos.
- [ ] **rw-007**: Sección Backups: schedule + retención + comando restore.
- [ ] **rw-008**: Sección Update: flujo `git tag → CI → .env → pull/restart`.

## Validación

- [ ] **test-001**: `wc -l README.md` ≤ 100 líneas.
- [ ] **test-002**: Cero menciones de "dominio", "TLS", "https", "sslip",
      "cloudflare", "Let's Encrypt".
- [ ] **test-003**: Cero secretos o ejemplos con credenciales reales.
- [ ] **test-004**: Diagrama renderiza correctamente en GitHub preview.
- [ ] **test-005**: Todos los comandos `make` mencionados existen en
      `Makefile` post HU-38.8.
- [ ] **test-006**: Todas las URLs mencionadas resuelven correctamente
      con stack arriba.
- [ ] **test-007**: README leíble de arriba abajo en <2 min.
- [ ] **test-008**: Markdown válido (sin headers rotos, listas mal
      formateadas).

## Notas para reviewers

- SOLO se edita `README.md` (raíz de la rama services).
- Esta HU es la ÚLTIMA wave; depende de que Makefile (HU-38.8) e
  install.sh (HU-38.10) estén finalizados con los nombres correctos
  de targets y comandos.
- Sin emojis, sin colores ASCII, sin gráficos pesados.
- Tono profesional, directo, español.
