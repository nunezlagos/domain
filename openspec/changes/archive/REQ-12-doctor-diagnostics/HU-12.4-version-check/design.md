# Design: HU-12.4-version-check

## Decisión arquitectónica

### Version checker

```go
// internal/version/check.go
package version

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

const (
    DefaultCheckFrequency = 1 * time.Hour
    GitHubAPIURL          = "https://api.github.com/repos/nunezlagos/memoria/releases/latest"
)

type GitHubRelease struct {
    TagName    string `json:"tag_name"`
    Prerelease bool   `json:"prerelease"`
}

type CheckResult struct {
    CurrentVersion  string `json:"current_version"`
    LatestVersion   string `json:"latest_version,omitempty"`
    UpdateAvailable bool   `json:"update_available"`
    CheckedAt       string `json:"checked_at,omitempty"`
    Error           string `json:"error,omitempty"`
}
```

### VersionChecker with cache

```go
// internal/version/check.go
type VersionChecker struct {
    client         *http.Client
    checkFrequency time.Duration
    cache          *checkCache
}

type checkCache struct {
    result    *CheckResult
    checkedAt time.Time
}

func NewVersionChecker() *VersionChecker {
    return &VersionChecker{
        client:         &http.Client{Timeout: 10 * time.Second},
        checkFrequency: DefaultCheckFrequency,
        cache:          &checkCache{},
    }
}

func (vc *VersionChecker) Check(ctx context.Context, force bool) *CheckResult {
    // Return cached result if fresh and not forced
    if !force && vc.cache.result != nil {
        if time.Since(vc.cache.checkedAt) < vc.checkFrequency {
            return vc.cache.result
        }
    }

    result := vc.doCheck(ctx)

    // Update cache regardless of result (avoid retry spam on errors)
    vc.cache.result = result
    vc.cache.checkedAt = time.Now()

    return result
}

func (vc *VersionChecker) doCheck(ctx context.Context) *CheckResult {
    req, err := http.NewRequestWithContext(ctx, "GET", GitHubAPIURL, nil)
    if err != nil {
        return &CheckResult{CurrentVersion: Version, Error: "request creation failed"}
    }
    req.Header.Set("Accept", "application/vnd.github.v3+json")

    resp, err := vc.client.Do(req)
    if err != nil {
        return &CheckResult{CurrentVersion: Version, Error: "offline"}
    }
    defer resp.Body.Close()

    if resp.StatusCode == 403 {
        return &CheckResult{CurrentVersion: Version, Error: "rate limited"}
    }
    if resp.StatusCode != 200 {
        return &CheckResult{CurrentVersion: Version,
            Error: fmt.Sprintf("GitHub API returned %d", resp.StatusCode)}
    }

    var release GitHubRelease
    if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
        return &CheckResult{CurrentVersion: Version, Error: "invalid response"}
    }

    // Skip pre-releases
    if release.Prerelease {
        return &CheckResult{CurrentVersion: Version, Error: "latest is pre-release"}
    }

    result := &CheckResult{
        CurrentVersion: Version,
        LatestVersion:  release.TagName,
        UpdateAvailable: release.TagName != Version,
        CheckedAt:      time.Now().UTC().Format(time.RFC3339),
    }
    return result
}
```

### Version command integration

```go
// internal/cli/version.go (modified)
var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Show version and update information",
    RunE: func(cmd *cobra.Command, args []string) error {
        forceCheck, _ := cmd.Flags().GetBool("check")
        asJSON, _ := cmd.Flags().GetBool("json")

        info := version.GetInfo()

        if asJSON {
            result := version.NewVersionChecker().Check(cmd.Context(), forceCheck)
            return printJSON(struct {
                version.Info
                Update *version.CheckResult `json:"update,omitempty"`
            }{info, result})
        }

        fmt.Printf("memoria %s (%s) %s\n", info.Version, info.Commit, info.BuildDate)
        fmt.Printf("go%s %s/%s\n", info.GoVersion, info.OS, info.Arch)

        result := version.NewVersionChecker().Check(cmd.Context(), forceCheck)
        printUpdateStatus(result)

        return nil
    },
}

func printUpdateStatus(r *version.CheckResult) {
    switch {
    case r.Error == "offline":
        fmt.Println("⚠ version check failed (offline)")
    case r.Error == "rate limited":
        fmt.Println("⚠ version check failed (rate limited)")
    case r.Error != "":
        fmt.Printf("⚠ version check failed (%s)\n", r.Error)
    case r.UpdateAvailable:
        fmt.Printf("⚠ update available: %s\n", r.LatestVersion)
        fmt.Printf("  Run `go install github.com/nunezlagos/memoria@latest` to update\n")
    default:
        fmt.Println("✓ up-to-date")
    }
}
```

### CheckFrequency configuration

```go
// Default is fine for most users; can be changed for testing
func SetCheckFrequency(d time.Duration) {
    DefaultCheckFrequency = d
}
```

### Version info extension

```go
// internal/version/version.go (modified)
type Info struct {
    Version   string `json:"version"`
    Commit    string `json:"commit"`
    BuildDate string `json:"build_date"`
    GoVersion string `json:"go_version"`
    OS        string `json:"os"`
    Arch      string `json:"arch"`
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| GitHub API con autenticación | Sin auth son 60 req/h; suficiente para uso normal; auth complica setup |
| Polling en background | Consume recursos; version check es bajo demanda |
| Cache en disco | In-memory es suficiente; se pierde al reiniciar pero es aceptable |
| Semver parsing estricto | Comparación string vs tag_name es suficiente; semver no es necesario para notificación |

## TDD plan

1. **Red:** Version checker retorna up-to-date para misma versión → falla
2. **Green:** Implement doCheck mockeando HTTP → pasa
3. **Red:** Update available detecta versión distinta → falla
4. **Green:** Comparación tag_name != Version → pasa
5. **Red:** Cache retorna resultado sin HTTP request → falla
6. **Green:** Implement cache con time check → pasa
7. **Red:** Offline no bloquea CLI → falla
8. **Green:** Error graceful → pasa
9. **Sabotaje:** No ignorar pre-releases → muestra rc como latest → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| GitHub API rate limiting | Cache de 1h minimiza requests; error graceful si rate limited |
| Cambio de URL de API | Error "GitHub API returned X" reporta código exacto |
| Tag name no sigue semver | Comparación string exacta; user ve el tag_name real |
| Timeout de red lento | HTTP client timeout 10s; error offline rápido |
