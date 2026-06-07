# Design: HU-09.8-sync-guidance

## Decisión arquitectónica

### Error codes

```go
// internal/cloud/sync/errors.go
package sync

type SyncError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Detail  string `json:"detail,omitempty"`
}

func (e SyncError) Error() string { return e.Message }

// Known error codes
const (
    CodeAuthExpired     = "auth_expired"
    CodeNetworkTimeout  = "network_timeout"
    CodeSyncConflict    = "sync_conflict"
    CodeRateLimited     = "rate_limited"
    CodeInternalError   = "internal_error"
    CodeUnknown         = "unknown"
)
```

### IsRepairableCloudSyncError

```go
// internal/cloud/sync/guidance.go
var repairableCodes = map[string]bool{
    CodeAuthExpired:    true,
    CodeNetworkTimeout: true,
    CodeSyncConflict:   true,
    CodeRateLimited:    true,
}

func IsRepairableCloudSyncError(err error) bool {
    var syncErr SyncError
    if errors.As(err, &syncErr) {
        return repairableCodes[syncErr.Code]
    }
    return false
}
```

### BuildGuidance

```go
// internal/cloud/sync/guidance.go
type GuidanceMessage struct {
    Title       string
    Description string
    Steps       []string
    Commands    []string
}

func BuildGuidance(err error) string {
    var syncErr SyncError
    if !errors.As(err, &syncErr) {
        return ""
    }

    msg := guidanceForCode(syncErr.Code)
    if msg == nil {
        return ""
    }

    msg.Description = fillDetail(msg.Description, syncErr.Detail)

    var b strings.Builder
    b.WriteString(fmt.Sprintf("## %s\n\n", msg.Title))
    b.WriteString(fmt.Sprintf("%s\n\n", msg.Description))
    b.WriteString("### Steps to resolve\n\n")
    for i, step := range msg.Steps {
        b.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
    }
    b.WriteString("\n### Commands\n\n")
    for _, cmd := range msg.Commands {
        b.WriteString(fmt.Sprintf("  `%s`\n", cmd))
    }

    return b.String()
}
```

### Guidance templates

```go
// internal/cloud/sync/guidance_templates.go
func guidanceForCode(code string) *GuidanceMessage {
    switch code {
    case CodeAuthExpired:
        return &GuidanceMessage{
            Title: "Authentication Expired",
            Description: "Your cloud sync authentication token has expired. Re-authentication is required.",
            Steps: []string{
                "Run the authentication command below",
                "Follow the OAuth flow in your browser",
                "Retry the sync operation",
            },
            Commands: []string{"engram cloud auth login"},
        }

    case CodeNetworkTimeout:
        return &GuidanceMessage{
            Title: "Network Timeout",
            Description: "The sync operation timed out. This may be due to network issues or a slow connection.",
            Steps: []string{
                "Check your internet connection",
                "Verify the cloud server is reachable",
                "Retry the sync operation",
                "If the issue persists, run diagnostics",
            },
            Commands: []string{
                "engram doctor",
                "engram cloud sync",
            },
        }

    case CodeSyncConflict:
        return &GuidanceMessage{
            Title: "Sync Conflict Detected",
            Description: "Local and remote data have diverged. Manual resolution is required.",
            Steps: []string{
                "Run diagnostics to identify conflicts",
                "Review the conflicting observations",
                "Apply the recommended repair",
                "Retry the sync",
            },
            Commands: []string{
                "engram doctor --sync",
                "engram repair --auto",
                "engram cloud sync",
            },
        }

    case CodeRateLimited:
        return &GuidanceMessage{
            Title: "Rate Limited",
            Description: "You have exceeded the API rate limit. Please wait before retrying.",
            Steps: []string{
                "Wait for the rate limit window to reset",
                "Reduce sync frequency if you are a power user",
                "Check your current rate limit status",
            },
            Commands: []string{
                "engram cloud status",
                "engram doctor",
            },
        }

    default:
        return nil
    }
}

func fillDetail(template, detail string) string {
    if detail == "" {
        return template
    }
    return template + "\n\nDetails: " + detail
}
```

### Integration point

```go
// Called from sync handler when error occurs
func handleSyncError(err error) {
    if IsRepairableCloudSyncError(err) {
        guidance := BuildGuidance(err)
        log.Warn("Sync error (repairable): ", err.Error())
        fmt.Println(guidance) // or send to TUI/notification
    } else {
        log.Error("Sync error (non-repairable): ", err.Error())
    }
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Guidance en JSON externo | Más flexible pero overkill para pocos códigos; hardcodeado es más simple y testeable |
| Auto-ejecutar doctor/repair | El usuario debe decidir; solo guidance por ahora |
| GUIDs en vez de códigos string | String codes son más legibles en logs y debugging |

## TDD plan

1. **Red:** IsRepairableCloudSyncError(auth_expired) retorna true → falla
2. **Green:** Implement classify function → pasa
3. **Red:** IsRepairableCloudSyncError(internal_error) retorna false → falla
4. **Green:** Default false para no mapeados → pasa
5. **Red:** BuildGuidance retorna mensaje formateado → falla
6. **Green:** Implement guidance builders → pasa
7. **Red:** Non-repairable error retorna empty string → falla
8. **Green:** Implement default return → pasa
9. **Sabotaje:** No verificar errors.As → nil pointer en guidance → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Error code no coincide con server | Documentar códigos en ambos lados; test de integración |
| Guidance se desactualiza | Template functions centralizadas; fácil de actualizar |
| Mensajes muy largos en TUI | Guidance truncado a N chars en TUI; completo en logs |
