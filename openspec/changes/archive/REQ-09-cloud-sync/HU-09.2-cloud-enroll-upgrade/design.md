# Design: HU-09.2-cloud-enroll-upgrade

## Decisión arquitectónica

### State machine

```go
type CloudState string

const (
    StateNone       CloudState = "none"
    StateConfigured CloudState = "configured"
    StateEnrolled   CloudState = "enrolled"
    StateUpgraded   CloudState = "upgraded"
    StateError      CloudState = "error"
)

var validTransitions = map[CloudState][]CloudState{
    StateNone:       {StateConfigured},
    StateConfigured: {StateEnrolled, StateNone},
    StateEnrolled:   {StateUpgraded, StateError, StateConfigured},
    StateUpgraded:   {StateEnrolled, StateError},
    StateError:      {StateConfigured, StateEnrolled},
}

func (s CloudState) CanTransitionTo(target CloudState) bool {
    for _, t := range validTransitions[s] {
        if t == target {
            return true
        }
    }
    return false
}
```

### Enrollment

```go
type EnrollRequest struct {
    MachineID string `json:"machine_id"`
    Hostname  string `json:"hostname"`
    Version   string `json:"version"`
}

type EnrollResponse struct {
    EnrollmentID string `json:"enrollment_id"`
    ServerVersion string `json:"server_version"`
}

func Enroll(ctx context.Context, config *CloudConfig) (*EnrollResponse, error) {
    if config.State != StateConfigured && config.State != StateError {
        return nil, fmt.Errorf("cannot enroll from state %s", config.State)
    }
    req := EnrollRequest{
        MachineID: getMachineID(),
        Hostname:  getHostname(),
        Version:   version.Version,
    }
    var resp EnrollResponse
    if err := cloudPost(ctx, config, "/api/enroll", req, &resp); err != nil {
        config.State = StateError
        SaveCloudConfig(config)
        return nil, err
    }
    config.EnrollmentID = resp.EnrollmentID
    config.State = StateEnrolled
    config.EnrolledAt = time.Now().UTC().Format(time.RFC3339)
    SaveCloudConfig(config)
    return &resp, nil
}
```

### Doctor checks

```go
type DoctorCheck struct {
    Name    string `json:"name"`
    Status  string `json:"status"` // "pass", "fail", "warn"
    Message string `json:"message,omitempty"`
}

var doctorChecks = []struct{
    Name string
    Check func(ctx context.Context, config *CloudConfig) DoctorCheck
}{
    {"config_exists", checkConfigExists},
    {"server_reachable", checkServerReachable},
    {"token_valid", checkTokenValid},
    {"enrollment_active", checkEnrollmentActive},
}
```

### Upgrade subcommands

```
engram upgrade doctor    → ejecuta checks, reporta JSON/texto
engram upgrade repair    → doctor + auto-fix (re-token, re-enroll)
engram upgrade bootstrap → wizard interactivo paso a paso
engram upgrade rollback  → cloud.json.bak → cloud.json
engram upgrade status    → mustra estado actual resumido
```

### Backup strategy

```go
func backupConfig(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err // no existe, no hay backup
    }
    return os.WriteFile(path+".bak", data, 0600)
}
```

Rollback:
```go
func rollbackConfig(path string) error {
    bak := path + ".bak"
    if _, err := os.Stat(bak); os.IsNotExist(err) {
        return fmt.Errorf("no backup found to rollback to")
    }
    data, _ := os.ReadFile(bak)
    return os.WriteFile(path, data, 0600)
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| State machine con librería externa | Pocos estados y transiciones; switch manual es más claro |
| Bootstrap sin wizard | Mala UX para nuevos usuarios; wizard reduce fricción |

## TDD plan

1. **Red:** Enroll registra instancia → falla
2. **Green:** Implement POST /api/enroll → pasa
3. **Red:** Doctor checks retorna reporte → falla
4. **Green:** Implement 4 checks → pasa
5. **Red:** Rollback requiere backup → falla
6. **Green:** Implement backup + rollback → pasa
7. **Sabotaje:** Romper state transition (permitir enrolled→none) → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Machine ID no disponible (`/etc/machine-id`) | Fallback a `hostid` + hash; si no, generar UUID y persistir |
| Server rechaza enrollment (token inválido) | Error claro + sugerencia de re-config |
| Rollback sin backup | Error "no backup found"; no operación destructiva sin respaldo |
