// Dep check + auto-install (HU-01.11).
//
// Flujo:
//   1. Llamador pide `Check(neededDeps)` → []Result
//   2. Para cada Dep faltante, llama `OfferInstall(pm, dep, withConfirm)`
//   3. Si el user confirma, ejecuta el install command
//   4. Re-chequea para confirmar
//
// El callback `withConfirm` desacopla la UI: TUI bubbletea, CLI bufio,
// o test puro pueden implementarlo diferente.

package installer

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// versionRegexp matchea semver basico: X.Y.Z (con componentes opcionales).
// Acepta prefijos 'v' o 'go' (case-insensitive para el segundo).
var versionRegexp = regexp.MustCompile(`(?:v|go)?\d+\.\d+(?:\.\d+)?(?:[-+][0-9A-Za-z.\-]+)?`)

// Dep es una dependencia a chequear/instalar.
type Dep struct {
	Name    string // "go", "git", "docker"
	Binary  string // nombre del binario en PATH (e.g., "go", "git")
	MinVer  string // version minima requerida (e.g., "1.22"); "" = sin min
	PkgName string // nombre del paquete en el pkg manager (puede diferir del Binary)
}

// Well-known deps que Domain necesita.
var (
	DepGo     = Dep{Name: "go", Binary: "go", MinVer: "1.22", PkgName: "golang-go"}
	DepGit    = Dep{Name: "git", Binary: "git", PkgName: "git"}
	DepDocker = Dep{Name: "docker", Binary: "docker", PkgName: "docker.io"}
)

// CheckResult es el resultado de chequear una Dep.
type CheckResult struct {
	Dep      Dep
	Found    bool   // el binario esta en PATH
	Path     string // path absoluto del binario
	Version  string // version reportada ("" si no se pudo parsear)
	MinMet   bool   // true si Found AND version >= MinVer
	Hint     string // comando sugerido para el user si Found=false
}

// Check corre `which <binary>` y opcionalmente `<binary> --version`
// para todas las deps. Retorna un slice con el resultado.
func Check(deps []Dep) []CheckResult {
	results := make([]CheckResult, len(deps))
	for i, d := range deps {
		results[i] = checkOne(d)
	}
	return results
}

func checkOne(d Dep) CheckResult {
	r := CheckResult{Dep: d}
	if d.PkgName == "" {
		r.Dep.PkgName = d.Binary
	}
	path, err := exec.LookPath(d.Binary)
	if err != nil {
		r.Found = false
		r.Hint = installHint(d)
		return r
	}
	r.Found = true
	r.Path = path
	if d.MinVer != "" {
		v, err := getVersion(d.Binary)
		if err == nil {
			r.Version = v
			r.MinMet = compareVersion(v, d.MinVer) >= 0
		}
	} else {
		r.MinMet = true
	}
	return r
}

// versionCmds es el orden de intentos para extraer version de un binary.
// Algunos binarios (go) usan `version`, otros (git) usan `--version`.
var versionCmds = [][]string{
	{"--version"},
	{"version"},
	{"-version"},
	{"-V"},
}

// getVersion ejecuta `<binary> <args>...` y extrae la version.
// Usa regex para ser tolerante con formatos como:
//   - "go version go1.22.3 linux/amd64"
//   - "go version go1.26.4-X:nodwarf5 linux/amd64"
//   - "git version 2.43.0"
//   - "Docker version 20.10.21, build ..."
// Retorna el primer match que parezca semver (X.Y.Z).
// Si ninguno de los comandos标准的 retorna output parseable,
// retorna ("", nil) — caller decide si es error o no.
func getVersion(binary string) (string, error) {
	for _, args := range versionCmds {
		out, err := exec.Command(binary, args...).Output()
		if err != nil {
			continue // probar siguiente
		}
		s := strings.TrimSpace(string(out))
		if s == "" {
			continue
		}
		// Regex: digitos.puntos.digitos (opcional mas componentes)
		match := versionRegexp.FindString(s)
		if match != "" {
			match = strings.TrimPrefix(match, "v")
			match = strings.TrimPrefix(match, "go")
			return match, nil
		}
	}
	return "", nil
}

// compareVersion compara "1.22.3" vs "1.22" semver-ish.
// Retorna -1, 0, 1.
func compareVersion(have, want string) int {
	hParts := strings.Split(have, ".")
	wParts := strings.Split(want, ".")
	for i := 0; i < len(wParts); i++ {
		var h, w int
		if i < len(hParts) {
			fmt.Sscanf(hParts[i], "%d", &h)
		}
		fmt.Sscanf(wParts[i], "%d", &w)
		if h < w {
			return -1
		}
		if h > w {
			return 1
		}
	}
	return 0
}

// installHint retorna el comando sugerido para instalar manualmente.
func installHint(d Dep) string {
	pm, _ := DetectPlatform()
	return InstallCommand(pm.PkgMgr, d)
}

// InstallCommand retorna el comando shell para instalar d via pm.
// NO lo ejecuta. Solo lo retorna como string para mostrar al user.
func InstallCommand(pm PkgManager, d Dep) string {
	pkgName := d.PkgName
	if pkgName == "" {
		pkgName = d.Binary
	}
	switch pm {
	case PkgApt:
		return fmt.Sprintf("sudo apt install -y %s", pkgName)
	case PkgDnf:
		return fmt.Sprintf("sudo dnf install -y %s", pkgName)
	case PkgYum:
		return fmt.Sprintf("sudo yum install -y %s", pkgName)
	case PkgPacman:
		return fmt.Sprintf("sudo pacman -S --noconfirm %s", pkgName)
	case PkgApk:
		return fmt.Sprintf("sudo apk add %s", pkgName)
	case PkgBrew:
		return fmt.Sprintf("brew install %s", pkgName)
	case PkgChoco:
		return fmt.Sprintf("choco install -y %s", pkgName)
	case PkgWinget:
		return fmt.Sprintf("winget install -y %s", pkgName)
	}
	return fmt.Sprintf("(no install command for pkg manager %s)", pm)
}

// ConfirmFunc es el callback que pide confirmacion al user.
// Retorna true si el user acepta, false si rechaza.
type ConfirmFunc func(prompt string) bool

// Install ejecuta el install de d via el package manager del Platform.
// Si withConfirm retorna false, aborta sin ejecutar nada.
//
// Retorna nil si el install fue exitoso o si fue skipped por el user
// (con skipped=true). Retorna error si la ejecucion fallo.
func Install(ctx context.Context, pm Platform, d Dep, withConfirm ConfirmFunc) (skipped bool, err error) {
	if withConfirm == nil {
		return false, errors.New("withConfirm is nil")
	}
	cmd := InstallCommand(pm.PkgMgr, d)
	prompt := fmt.Sprintf("Need to install %s via %s.\n  Command: %s\nProceed? [Y/n]: ", d.Name, pm.PkgMgr, cmd)
	if !withConfirm(prompt) {
		return true, nil
	}
	return false, runInstallCommand(ctx, cmd)
}

// runInstallCommand parsea el cmd string y lo ejecuta.
// Implementacion naive: split por whitespace. Suficiente para los
// comandos canonicos que generamos.
func runInstallCommand(ctx context.Context, cmdStr string) error {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return errors.New("empty install command")
	}
	// Si el primer arg es sudo, mantenerlo.
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install failed: %w\noutput: %s", err, string(out))
	}
	return nil
}
