# Design: issue-29.4-opencode-json-untracked-permanent

## Contexto

`opencode.json` es un archivo LOCAL del developer: contiene paths
absolutos del home (`~/.config/opencode/opencode.json` en producción,
`/Users/x/code/.../opencode.json` en dev) y, en escenarios futuros,
puede llevar la `DOMAIN_API_KEY` (el user puede pegarla directo en
config de opencode en vez de via env var).

El 2026-06-12 se commiteó accidentalmente `opencode.json` (con paths
absolutos del repo) y se tuvo que revertir + agregar al `.gitignore`.
La línea 36-39 del `.gitignore` actual ya tiene las 4 entradas:

```
opencode.json
opencode.json.backup-*
.mcp.json
.mcp.json.backup-*
```

El issue es: **blindar esto con un test de regresión** para que un
futuro PR que remueva esas líneas (o que use `git add -f`) sea
detectado automáticamente en CI.

## Decisión arquitectónica

**Estrategia:** test de regresión que corre `git ls-files` y verifica
que los archivos prohibidos no estén en el index.

1. Test `TestOpencodeJSONNotTracked` en
   `internal/cli/install/gitignore_guard_test.go` (nuevo archivo).
2. Helper `assertNotTracked(t *testing.T, paths ...string)` que corre
   `git ls-files --error-unmatch <path>` para cada path y asserta exit
   code != 0 (lo que indica que NO está tracked; si está tracked,
   `git ls-files` lo imprime y exit 0).
3. Test paralelo `assertGitignoreHasEntry` que verifica que las
   entradas siguen en `.gitignore` (grepea el archivo). Defensa en
   profundidad: aunque alguien borrara el archivo del index, no
   podría borrar la línea del `.gitignore` sin romper el test.
4. CI: el test corre automáticamente con `go test ./...`. No requiere
   workflow nuevo.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Pre-commit hook (lefthook, pre-commit) | Mencionado en la tabla del REQ como "opcional", pero requiere setup per-developer. El test de CI es enforcement centralizado. |
| B | Git attribute `export-ignore` (afecta `git archive`, no tracking) | No resuelve el problema: queremos阻止 que se commitee, no excluirlo del tarball. |
| C | Limpiar el archivo del index en cada CI run | Tapa el problema en vez de prevenirlo. Si el archivo tiene secretos reales, ya se filtraron al log de git. |
| D | `.gitignore` global del developer (`git config --global core.excludesFile`) | No aplica: el `.gitignore` DEBE estar en el repo para que aplique a todos los clones. |

## Por qué A (test de regresión) gana

- **Cero infraestructura nueva:** solo un test Go + un helper.
- **Corre automáticamente en CI:** cualquier PR que toque `.gitignore`
  o `opencode.json` se testea.
- **Defensa en profundidad:** verifica TANTO el `.gitignore` (grepeando
  las entradas) COMO el index de git (`git ls-files`).
- **Sabotaje-testable:** el sabotaje natural (borrar la línea del
  `.gitignore`) está cubierto.

## Detalle de implementación

```go
// internal/cli/install/gitignore_guard_test.go

package install_test

import (
    "os/exec"
    "strings"
    "testing"
)

func TestOpencodeJSONNotTracked(t *testing.T) {
    cases := []string{
        "opencode.json",
        ".mcp.json",
    }
    for _, p := range cases {
        t.Run(p, func(t *testing.T) {
            // git ls-files --error-unmatch <path>
            // exit 0 si está tracked (MAL), exit 1 si NO está tracked (BIEN)
            cmd := exec.Command("git", "ls-files", "--error-unmatch", p)
            out, err := cmd.CombinedOutput()
            if err == nil {
                t.Fatalf("%s está tracked en git pero NO debería:\n%s", p, out)
            }
        })
    }
}

func TestGitignoreHasLocalConfigEntries(t *testing.T) {
    requiredEntries := []string{
        "opencode.json",
        ".mcp.json",
    }
    data, err := os.ReadFile("../../.gitignore") // ajustar path
    if err != nil { t.Fatal(err) }
    content := string(data)
    for _, entry := range requiredEntries {
        if !strings.Contains(content, entry) {
            t.Errorf(".gitignore no contiene %q — agregalo de vuelta", entry)
        }
    }
}
```

## Riesgos

- **R1:** El test asume que corre dentro de un git repo. **Mitigación:**
  `t.Skip` si `git rev-parse --git-dir` falla.
- **R2:** Si en el futuro se decide commitear un `opencode.json.example`
  (template), el test podría romperse. **Mitigación:** el grep del
  `.gitignore` busca `opencode.json` exacto, no `opencode.json.example`.
  Si se agrega el template, hay que actualizar el test explícitamente
  con un comentario del porqué.

## Sabotaje test (referencia)

Romper el `.gitignore` (borrar las 4 líneas) o forzar `git add -f
opencode.json` en un branch sabotaje → test DEBE FALLAR → restaurar.
