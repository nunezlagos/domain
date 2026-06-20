// Package workflowimport — Domain MCP reemplaza los archivos .md de
// instrucciones IA (Claude Code, OpenCode, Cursor, Windsurf, Aider) por
// stubs que apuntan al MCP de Domain. Los originales se guardan en BD
// para audit + rollback.
//
// Es la pieza que hace que Domain sea PLUG-AND-PLAY real: el usuario
// instala el MCP y "domain init" sobrescribe los .md → próxima vez que
// abre el agente IA, el contexto del proyecto viene de Domain MCP en
// lugar de los .md sueltos.
package workflowimport

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Scanner recorre el filesystem buscando archivos .md de instrucciones
// IA según patterns conocidos. NO escribe nada — solo detecta.
type Scanner struct {
	ProjectRoot string
}

// DetectedFile describe un archivo encontrado por el scanner.
type DetectedFile struct {
	RelPath     string `json:"rel_path"`
	SourceTool  string `json:"source_tool"`   // claude-code | opencode | cursor | windsurf | aider | generic
	SizeBytes   int64  `json:"size_bytes"`
	ContentHash string `json:"content_hash"`
	Content     string `json:"content,omitempty"` // populado solo si SnapshotContent=true
}

var (
	// ErrRootNotFound se devuelve si ProjectRoot no existe.
	ErrRootNotFound = errors.New("project root not found")
)

// Detect escanea el proyecto y devuelve la lista de archivos .md de
// instrucciones IA encontrados. Lookup paths:
//
//   - <root>/CLAUDE.md / claude.md
//   - <root>/.claude/**/*.md
//   - <root>/.opencode/**/*.md
//   - <root>/.cursor/**/*.md / <root>/.cursorrules
//   - <root>/.windsurfrules / <root>/.windsurf/**/*.md
//   - <root>/CONVENTIONS.md / AGENTS.md (genéricos)
//
// snapshotContent: si true, lee el contenido completo a cada DetectedFile.
// Si false, solo metadata (mucho más rápido en repos grandes).
func (s *Scanner) Detect(snapshotContent bool) ([]DetectedFile, error) {
	if s.ProjectRoot == "" {
		s.ProjectRoot = "."
	}
	if _, err := os.Stat(s.ProjectRoot); err != nil {
		return nil, ErrRootNotFound
	}

	patterns := []struct {
		match func(rel string) bool
		tool  string
	}{
		{matchRoot("CLAUDE.md"), "claude-code"},
		{matchRoot("claude.md"), "claude-code"},
		{matchDirRegex(".claude", ".md"), "claude-code"},
		{matchDirRegex(".opencode", ".md"), "opencode"},
		{matchDirRegex(".cursor", ".md"), "cursor"},
		{matchRoot(".cursorrules"), "cursor"},
		{matchRoot(".windsurfrules"), "windsurf"},
		{matchDirRegex(".windsurf", ".md"), "windsurf"},
		{matchRoot(".aider.conf.yml"), "aider"},
		{matchRoot("CONVENTIONS.md"), "generic"},
		{matchRoot("AGENTS.md"), "generic"},
		{matchRoot("AGENT.md"), "generic"},
	}

	out := []DetectedFile{}
	err := filepath.WalkDir(s.ProjectRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		// Skip dirs irrelevantes
		if d.IsDir() {
			base := filepath.Base(path)
			if base == "node_modules" || base == "vendor" || base == ".git" {
				return filepath.SkipDir
			}
			// Permitir .claude / .opencode / .cursor / .windsurf
			if strings.HasPrefix(base, ".") && !isAllowedHiddenDir(base) && path != s.ProjectRoot {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(s.ProjectRoot, path)
		rel = filepath.ToSlash(rel)

		for _, p := range patterns {
			if p.match(rel) {
				info, err := d.Info()
				if err != nil {
					return nil
				}
				df := DetectedFile{
					RelPath:    rel,
					SourceTool: p.tool,
					SizeBytes:  info.Size(),
				}
				if snapshotContent && info.Size() < 5*1024*1024 {
					data, err := os.ReadFile(path)
					if err == nil {
						df.Content = string(data)
						sum := sha256.Sum256(data)
						df.ContentHash = hex.EncodeToString(sum[:])
					}
				} else {
					// Solo hash sin guardar contenido en struct.
					data, err := os.ReadFile(path)
					if err == nil {
						sum := sha256.Sum256(data)
						df.ContentHash = hex.EncodeToString(sum[:])
					}
				}
				out = append(out, df)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out, nil
}

func matchRoot(name string) func(string) bool {
	return func(rel string) bool {
		return rel == name
	}
}

func matchDirRegex(dirPrefix, ext string) func(string) bool {
	return func(rel string) bool {
		return strings.HasPrefix(rel, dirPrefix+"/") && strings.HasSuffix(rel, ext)
	}
}

func isAllowedHiddenDir(base string) bool {
	switch base {
	case ".claude", ".opencode", ".cursor", ".windsurf":
		return true
	}
	return false
}
