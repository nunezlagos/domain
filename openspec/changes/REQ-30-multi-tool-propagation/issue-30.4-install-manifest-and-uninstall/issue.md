# issue-30.4-install-manifest-and-uninstall

**Origen:** `REQ-30-multi-tool-propagation`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** developer que instaló domain y configuró wrappers, hooks, manifests locales
**Quiero** tener un registro global de TODOS los archivos que el instalador tocó, con su hash original y nuevo
**Para** poder revertir limpio con `domain uninstall`, y para poder auditar con `domain status --installed` qué se modificó

## Criterios de aceptación

### Escenario 1: Manifest global se actualiza en cada install

```gherkin
Dado que corro `domain install` y el install toca:
  - ~/.zshrc (wrapper)
  - ~/.claude/settings.json (hook)
  - ~/Proyectos/foo/.domain/install-manifest.json (local, via auto-detect)
  - ~/.config/domain/credentials.json (creación)
Cuando termina el install
Entonces `~/.config/domain/install-manifest.json` contiene TODAS esas entradas con:
  - path (absoluto)
  - type (rcfile_append | claude_settings_merge | file_create | json_upsert | symlink)
  - before_hash, after_hash (SHA-256)
  - timestamp (RFC3339)
  - originating_issue (e.g. "30.2", "30.3", "30.1")
```

### Escenario 2: `domain status --installed` muestra el detalle

```gherkin
Dado que el manifest global tiene 5 entries de un install previo
Cuando corro `domain status --installed`
Entonces imprime una tabla legible con columnas: path | type | timestamp | originating_issue
Y al final, resumen: "5 files modified by domain (last install 2026-06-12T19:30Z)"
Y el comando NO requiere server (lee solo el manifest local)
```

### Escenario 3: `domain uninstall` revierte TODO limpio

```gherkin
Dado que el manifest global tiene 5 entries
Cuando corro `domain uninstall --yes`
Entonces el install revierte cada entry:
  - rcfile_append: remueve el bloque entre markers del .zshrc
  - claude_settings_merge: remueve la entry de domain del array SessionStart
  - file_create: borra el archivo (si y solo si el after_hash coincide con el hash actual — sino, skip con warning)
  - json_upsert: revierte el JSON al before_hash (usando el backup más reciente)
  - symlink: `os.Remove` el symlink
Y cada reversión loggea: "reverted <path> (type=<type>)"
Y al final, summary: "uninstalled 5 changes; 0 errors"
Y exit code 0
```

### Escenario 4: `domain uninstall` es defensivo — no pisa si algo cambió

```gherkin
Dado que el manifest dice que `~/.zshrc` tenía after_hash=X
Y el `~/.zshrc` actual tiene hash Y != X (el user lo editó a mano)
Cuando corro `domain uninstall`
Entonces NO revierte ese archivo (skip con warning: "~/.zshrc modified externally; skipping revert")
Y los OTROS archivos sí se revierten
Y el manifest se actualiza para remover las entries revertidas
```

### Escenario 5: Manifest es append-only durante installs

```gherkin
Dado que el manifest ya tiene entries de installs previos
Cuando corro un nuevo `domain install`
Entonces el manifest AGREGA las nuevas entries al final (no borra las viejas)
Y cada entry tiene un `install_id` único (UUID) para distinguir runs
Y `domain status --installed` puede filtrar por install_id con `--install <id>`
```

### Escenario 6: Sabotaje — uninstall es destructivo sin confirmación

```gherkin
Dado que el manifest tiene entries legítimas
Y el código de uninstall NO tiene un confirm prompt (sabotaje)
Cuando corro `domain uninstall` accidentalmente
Entonces se revierten archivos sin pedir confirm
Y el test e2e que assserta "uninstall pide confirm antes de actuar" DEBE FALLAR
Cuando restauro el confirm prompt
Entonces el test verde
```

### Escenario 7: Edge case — manifest global corrupto

```gherkin
Dado que `~/.config/domain/install-manifest.json` tiene JSON inválido
Cuando corro `domain status --installed` o `domain uninstall`
Entonces el comando loggea warning "manifest corrupt; recreating from scratch"
Y `status` retorna empty list, `uninstall` retorna 0 (no hay nada que revertir)
Y NO crashea con panic
```

## Notas

- El manifest es la pieza clave para hacer `domain uninstall`
  SEGURO. Sin él, uninstall sería "rm -rf ~/.config/domain" +
 祈祷.
- Hash SHA-256 de archivos grandes: hacerlo en stream para no
  cargar todo en memoria.
- El manifest NO incluye secrets (credentials.json, .env). Solo
  metadatos: path + hash + tipo. El uninstall de credenciales debe
  ser un comando explícito separado (`domain uninstall
  --credentials`) con confirm doble.
