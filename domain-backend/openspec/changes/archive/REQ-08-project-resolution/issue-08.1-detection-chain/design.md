# Design: issue-08.1-detection-chain

## Decisión arquitectónica

### Pipeline: Chain of Responsibility

```go
type Strategy interface {
    Detect(ctx context.Context, path string) (*Result, error)
    Name() string
}

type Result struct {
    Value       string
    Source      string
    Confidence  float64
}

type Candidate struct {
    Value  string
    Source string
    Reason string // por qué fue descartado o por qué ganó
}

type DetectionReport struct {
    Project    string
    Source     string
    Confidence float64
    Steps      []StepResult
}

type StepResult struct {
    Strategy string
    Success  bool
    Value    string
    Error    string
}
```

El pipeline ejecuta strategies en orden:

1. **ConfigFile**: Busca `.engram/config.json` subiendo desde cwd hasta `/`. Lee campo `project`. Si existe y es non-empty → éxito con confidence 1.0.
2. **GitRemote**: Ejecuta `git config --get remote.origin.url`. Parse URL (strip `.git`, extract last path segment). Si hay resultado → éxito con confidence 0.9.
3. **GitRoot**: Ejecuta `git rev-parse --show-toplevel`. Usa basename del path resultante. Si es distinto de `"/"` → éxito con confidence 0.7.
4. **GitChild**: Solo si git root no dio nombre significativo (ej. repo root es `/` o nombre genérico). Lista subdirectorios depth=1 con restricciones. Usa `os.ReadDir` filtrado.
5. **Ambiguous**: Si múltiples candidatos con confidence similar → retorna error con sugerencias.
6. **DirBasename**: Usa `filepath.Base(cwd)`. Último recurso, confidence 0.3.

### Child scan con restricciones

```go
type ScanOpts struct {
    Depth     int
    MaxItems  int
    Timeout   time.Duration
    SkipDirs  []string
}

func scanDirCandidates(ctx context.Context, root string, opts ScanOpts) ([]string, error) {
    ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
    defer cancel()

    entries, err := os.ReadDir(root)
    if err != nil {
        return nil, err
    }

    var candidates []string
    for _, e := range entries {
        if len(candidates) >= opts.MaxItems {
            break
        }
        if !e.IsDir() {
            continue
        }
        if isNoiseDir(e.Name(), opts.SkipDirs) {
            continue
        }
        candidates = append(candidates, e.Name())
    }
    return candidates, ctx.Err()
}
```

### Noise directories

```go
var defaultNoiseDirs = []string{
    ".git", ".hg", ".svn",
    "node_modules", "vendor",
    "__pycache__", ".pyc",
    ".venv", "venv", "env",
    "dist", "build", "target",
    ".next", ".cache", ".sass-cache",
    "coverage", ".nyc_output",
    "bower_components", "jspm_packages",
    ".gradle", ".mvn",
    ".serverless", ".terraform",
}
```

### Git remote URL parsing

```go
func extractRepoName(rawURL string) string {
    // Soporta: git@github.com:user/repo.git
    //          https://github.com/user/repo.git
    //          https://github.com/user/repo
    //          ssh://git@host.com/user/repo.git
    // Extrae el último segmento del path, strip .git
    u, err := url.Parse(rawURL)
    if err != nil || u.Host == "" {
        // Formato SCP: git@host:path
        if strings.Contains(rawURL, "@") && strings.Contains(rawURL, ":") {
            parts := strings.Split(rawURL, ":")
            rawURL = "ssh://" + strings.Replace(parts[0], "@", "@", 1) + "/" + parts[1]
            u, _ = url.Parse(rawURL)
        }
    }
    if u == nil {
        return ""
    }
    path := strings.TrimSuffix(u.Path, ".git")
    path = strings.TrimPrefix(path, "/")
    segments := strings.Split(path, "/")
    return segments[len(segments)-1]
}
```

### Confidence thresholds

| Paso | Confidence | Condición de éxito |
|------|-----------|-------------------|
| ConfigFile | 1.0 | Archivo existe y `project` es non-empty |
| GitRemote | 0.9 | Remote URL parseable con nombre válido |
| GitRoot | 0.7 | Basename distinto de repo virtual |
| GitChild | 0.6 | Primer candidato matches algún heuristic |
| DirBasename | 0.3 | Siempre (fallback final) |

### Reporte final

`DetectProject()` retorna un `DetectionReport` con todos los pasos ejecutados (incluso los skipped porque un paso anterior ganó). El reporte incluye confidence y se usa downstream para normalización y warnings.

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Single regex-based detection | No permite reportar confidence ni candidates descartados; difícil de debuggear |
| Detectar solo por cwd | Ignora contexto git y config explícita; results inestables entre sesiones |
| Llamar git para cada paso | Una sola llamada `git rev-parse` + `git config` es más eficiente; cachear resultados |
| Child scan con `filepath.Walk` | Walk es recursivo; depth=1 es más simple y rápido con `os.ReadDir` directo |

## TDD plan

1. **Red:** Test de ConfigFile strategy: crear temp dir con `.engram/config.json` → espera "my-project" → falla
2. **Green:** Implementar `ConfigFileStrategy.Detect()` → pasa
3. **Red:** Test de GitRemote strategy: init repo + config remote → espera nombre extraído → falla
4. **Green:** Implementar `GitRemoteStrategy.Detect()` → pasa
5. **Red:** Test de child scan con noise dirs → asegura que node_modules no aparece → falla
6. **Green:** Implementar noise filter → pasa
7. **Red:** Test de timeout en child scan: contexto cancelado → espera error → falla
8. **Green:** Implementar context.WithTimeout → pasa
9. **Red:** Pipeline test: config file presente → pipeline no ejecuta git → falla
10. **Green:** Implementar pipeline loop con short-circuit → pasa
11. **Sabotaje:** Romper ConfigFile → pipeline debe caer a GitRemote → test de short-circuit

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| `os.ReadDir` lento en NFS/FUSE | Timeout 200ms con context; si expira, skip child scan y pasa a DirBasename |
| Git command no disponible | Si `exec.LookPath("git")` falla, Git strategies se marcan como unavailable |
| `.engram/config.json` malformado | JSON parse error loggeado; pipeline continúa al siguiente paso |
| Permisos insuficientes para leer directorio | Error capturado y tratado como "no candidate"; no bloquea el pipeline |
