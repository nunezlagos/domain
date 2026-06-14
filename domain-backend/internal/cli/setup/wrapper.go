// issue-30.2: shell wrapper (opencode + domain) que ejecuta
// `domain setup auto-detect "$PWD" --quiet` antes de delegar.
package setup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	WrapperMarkerOpen  = "# >>> domain-wrapper >>>"
	WrapperMarkerClose = "# <<< domain-wrapper <<<"
)

// WrapperSnippet es el bloque canónico de shell que se agrega al .zshrc/.bashrc.
const WrapperSnippet = `# >>> domain-wrapper >>>
# !! AUTO-GENERADO por domain install (issue-30.2). No editar a mano !!
domain() {
  case "$1" in
    setup|install|uninstall|status) command domain "$@"; return $? ;;
  esac
  command domain setup auto-detect "$PWD" --quiet 2>/dev/null
  command domain "$@"
}
opencode() {
  command domain setup auto-detect "$PWD" --quiet 2>/dev/null
  command opencode "$@"
}
# <<< domain-wrapper <<<`

func GenerateWrapperSnippet() string {
	return WrapperSnippet
}

// InstallShellWrapper agrega el snippet al rcfile si no contiene el marker.
// Retorna (true, nil) si instaló, (false, nil) si ya estaba.
func InstallShellWrapper(rcfile string) (bool, error) {
	data, err := os.ReadFile(rcfile)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read rcfile: %w", err)
	}

	if bytes.Contains(data, []byte(WrapperMarkerOpen)) {
		return false, nil
	}

	content := string(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n" + WrapperSnippet + "\n"

	if err := os.WriteFile(rcfile, []byte(content), 0o644); err != nil {
		return false, fmt.Errorf("write rcfile: %w", err)
	}

	return true, nil
}

// UninstallShellWrapper remueve el bloque entre markers del rcfile.
func UninstallShellWrapper(rcfile string) error {
	data, err := os.ReadFile(rcfile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read rcfile: %w", err)
	}

	content := string(data)
	openIdx := strings.Index(content, WrapperMarkerOpen)
	closeIdx := strings.Index(content, WrapperMarkerClose)
	if openIdx < 0 || closeIdx < 0 {
		return nil
	}

	// Include the newline before the open marker if present
	before := ""
	if openIdx > 0 && content[openIdx-1] == '\n' {
		before = content[:openIdx-1]
	} else {
		before = content[:openIdx]
	}

	closeEnd := closeIdx + len(WrapperMarkerClose)
	after := ""
	if closeEnd < len(content) && content[closeEnd] == '\n' {
		after = content[closeEnd+1:]
	} else if closeEnd < len(content) {
		after = content[closeEnd:]
	}

	result := before
	if before != "" && !strings.HasSuffix(before, "\n") {
		result += "\n"
	}
	result += after

	return os.WriteFile(rcfile, []byte(result), 0o644)
}

// HasWrapper chequea si el rcfile contiene el marker open.
func HasWrapper(rcfile string) (bool, error) {
	data, err := os.ReadFile(rcfile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read rcfile: %w", err)
	}
	return bytes.Contains(data, []byte(WrapperMarkerOpen)), nil
}

// DetectShell detecta la shell actual via $SHELL y retorna
// (shell_name, rcfile_path). Fallback a zsh.
func DetectShell() (string, string) {
	shellPath := os.Getenv("SHELL")
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}

	switch {
	case strings.HasSuffix(shellPath, "/zsh"):
		return "zsh", filepath.Join(home, ".zshrc")
	case strings.HasSuffix(shellPath, "/bash"):
		return "bash", filepath.Join(home, ".bashrc")
	default:
		return "zsh", filepath.Join(home, ".zshrc")
	}
}

func syntaxCheck(shell, rcfile string) error {
	var cmd *exec.Cmd
	switch shell {
	case "zsh":
		cmd = exec.Command("zsh", "-n", rcfile)
	case "bash":
		cmd = exec.Command("bash", "-n", rcfile)
	default:
		return nil
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s syntax error in %s: %s", shell, rcfile, string(out))
	}
	return nil
}
