# issue-29.4-opencode-json-untracked-permanent

**Origen:** `REQ-29-install-quick-fixes`
**Prioridad tentativa:** alta
**Tipo:** chore (guard permanent)

## Historia de usuario

**Como** mantenedor del repo `domain`
**Quiero** que `opencode.json` NUNCA pueda volver a ser commiteado al repo, ni siquiera por accidente
**Para** que no se filtren paths absolutos del home de un developer (reventaría `git pull` en otros clones) ni, en el futuro, API keys locales

## Criterios de aceptación

### Escenario 1: `opencode.json` sigue ignorado por git

```gherkin
Dado el estado actual del repo
Cuando corro `git check-ignore -v opencode.json`
Entonces git retorna la línea de `.gitignore` que matchea (`.gitignore:36:opencode.json`)
Y exit code 0
```

### Escenario 2: Test CI falla si el archivo vuelve a trackearse

```gherkin
Dado que un developer (o bot) corre `git add -f opencode.json && git commit`
Cuando corre el test de regresión `TestOpencodeJSONNotTracked`
Entonces el test FALLA con mensaje: "opencode.json está tracked pero debería estar en .gitignore"
Y el pipeline de CI se rompe
```

### Escenario 3: Mismo guard para `.mcp.json`

```gherkin
Dado que `.mcp.json` también está en `.gitignore` por la misma razón
Cuando corro el test de regresión
Entonces también verifica que `.mcp.json` no esté tracked
```

### Escenario 4: Sabotaje — remover la entrada de `.gitignore`

```gherkin
Dado que el test de regresión está en su lugar
Cuando un developer remueve las líneas `opencode.json`, `opencode.json.backup-*`,
`.mcp.json`, `.mcp.json.backup-*` del `.gitignore` (sabotaje)
Y commitea el cambio
Entonces el test `TestOpencodeJSONNotTracked` DEBE FALLAR (verifica que el ignore
existe Y que el archivo no está tracked)
Y el pipeline se rompe
```

### Escenario 5: Edge case — el archivo `opencode.json` local existe pero es ignorado

```gherkin
Dado que `opencode.json` existe en el cwd (creado por `domain setup opencode`)
Y NO está en el index de git
Cuando corro `git status`
Entonces `opencode.json` aparece como "Untracked files" o ignorado (con `--ignored`)
Y NUNCA aparece en "Changes to be committed"
```

## Notas

- `.gitignore` ya tiene las entradas correctas (líneas 36-39 del archivo
  actual). Este issue es **blindar la decisión con un test**, no agregar
  nada nuevo.
- El test debe correr en CI (issue ya cubierto por el Makefile target
  `test`).
- Pre-commit hook opcional (mencionado en la tabla del REQ) — fuera del
  scope de este issue, se puede agregar después si la fricción de CI no
  es suficiente.
