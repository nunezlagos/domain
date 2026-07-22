package acp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Workspace acota las operaciones de fs de una sesión ACP a un directorio raíz
// per-run. Toda ruta pedida por el agente pasa por resolve, que rechaza
// traversal (..), symlink-escape y rutas absolutas fuera del root.
type Workspace struct {
	root string
}

// NewWorkspace crea un directorio temporal como raíz del run. El cwd del run no
// es el del server, y las ops de fs DELEGADAS vía ACP se acotan a este root; no
// es un sandbox del subproceso opencode (mismo uid, ver DOMAINSERV-86).
func NewWorkspace() (*Workspace, error) {
	dir, err := os.MkdirTemp("", "acp-ws-*")
	if err != nil {
		return nil, fmt.Errorf("acp workspace: %w", err)
	}
	// canonizamos el root porque en macOS/tmp suele haber symlinks (/var → /private/var)
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("acp workspace resolve root: %w", err)
	}
	return &Workspace{root: resolved}, nil
}

// openWorkspace envuelve un directorio existente como root canónico (per-run
// provisto por el wiring). Canoniza symlinks para que resolve compare parejo.
func openWorkspace(root string) (*Workspace, error) {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("acp workspace open: %w", err)
	}
	return &Workspace{root: resolved}, nil
}

// Root devuelve la raíz absoluta del workspace.
func (w *Workspace) Root() string { return w.root }

// Cleanup borra el árbol del workspace. Idempotente.
func (w *Workspace) Cleanup() error {
	if w.root == "" {
		return nil
	}
	return os.RemoveAll(w.root)
}

// resolve valida y canoniza una ruta pedida por el agente contra el root.
// Devuelve la ruta absoluta canónica si cae dentro del root; error si escapa.
func (w *Workspace) resolve(path string) (string, error) {
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(w.root, abs)
	}
	resolved, err := w.evalExisting(filepath.Clean(abs))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(w.root, resolved)
	if err != nil {
		return "", fmt.Errorf("acp workspace: fuera del root: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("acp workspace: ruta %q escapa del root", path)
	}
	return resolved, nil
}

// evalExisting resuelve symlinks. Para rutas inexistentes (writes futuros que
// crean dirs nuevos) camina hacia arriba hasta el primer ancestro existente,
// lo canoniza y reensambla el resto: así detecta symlink-escape en cualquier
// nivel del árbol.
func (w *Workspace) evalExisting(abs string) (string, error) {
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	dir, rest := abs, ""
	for {
		parent := filepath.Dir(dir)
		rest = filepath.Join(filepath.Base(dir), rest)
		if parent == dir {
			return "", fmt.Errorf("acp workspace: sin ancestro resoluble para %q", abs)
		}
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			return filepath.Join(resolved, rest), nil
		}
		dir = parent
	}
}
