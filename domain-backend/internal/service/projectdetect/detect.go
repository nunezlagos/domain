// Package projectdetect — issue F2 auto-detect.
// Lee el CWD (o path dado) y extrae metadata del proyecto:
//   - git remote + branch actual
//   - stack (go, php, node, python, ruby)
//   - .domain/manifest.yaml si existe (source-of-truth)
//   - project_slug derivado del repo o del path
//
// Diseñado como función pura: Detect(path string) → *Metadata.
// Sin I/O de red, sin DB. La integración con DB vive en otra capa.

package projectdetect

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Stack string

const (
	StackUnknown Stack = "unknown"
	StackGo      Stack = "go"
	StackPHP     Stack = "php"
	StackNode    Stack = "node"
	StackPython  Stack = "python"
	StackRuby    Stack = "ruby"
)

type Metadata struct {
	SourcePath    string `json:"source_path"`
	ProjectSlug   string `json:"project_slug"`
	ProjectName   string `json:"project_name"`
	GitRemote     string `json:"git_remote"`     // "user/repo" cuando hay remote
	GitRemoteURL  string `json:"git_remote_url"` // URL cruda ("git@github.com:...")
	CurrentBranch string `json:"current_branch"` // "" si no es repo git
	Stack         Stack  `json:"stack"`
	HasManifest   bool   `json:"has_manifest"`
	ManifestPath  string `json:"manifest_path,omitempty"`
}

var ErrNotInProject = errors.New("not inside a project (no .git, no manifest, no recognized stack)")

// Detect analiza el path dado (default: CWD) y devuelve metadata.
// Si no encuentra nada utilizable, retorna ErrNotInProject.
func Detect(path string) (*Metadata, error) {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		path = cwd
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	m := &Metadata{SourcePath: abs, Stack: StackUnknown}

	root := findGitRoot(abs)
	if root != "" {
		gitDir := filepath.Join(root, ".git")
		rawURL, repo, _ := parseGitConfig(gitDir)
		branch, _ := readGitBranch(gitDir)
		m.GitRemoteURL = rawURL
		m.GitRemote = repo
		m.CurrentBranch = branch
	}

	m.Stack = detectStack(abs)
	m.HasManifest, m.ManifestPath = findManifest(abs)

	switch {
	case m.GitRemote != "":
		m.ProjectSlug = slugify(m.GitRemote)
		m.ProjectName = m.GitRemote
	case root != "":
		m.ProjectSlug = slugify(filepath.Base(root))
		m.ProjectName = filepath.Base(root)
	case m.HasManifest:
		base := filepath.Base(m.ManifestPath)
		m.ProjectSlug = slugify(strings.TrimSuffix(base, filepath.Ext(base)))
		m.ProjectName = m.ProjectSlug
	case m.Stack != StackUnknown:
		m.ProjectSlug = slugify(filepath.Base(abs))
		m.ProjectName = filepath.Base(abs)
	default:
		return nil, ErrNotInProject
	}

	return m, nil
}

func findGitRoot(start string) string {
	cur := start
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

func parseGitConfig(gitPath string) (remote, repo string, err error) {
	configPath := filepath.Join(gitPath, "config")
	f, err := os.Open(configPath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inRemote := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[remote ") {
			inRemote = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inRemote = false
			continue
		}
		if inRemote && strings.HasPrefix(line, "url = ") {
			remote = strings.TrimPrefix(line, "url = ")
			repo = repoFromRemote(remote)
			return remote, repo, nil
		}
	}
	return "", "", scanner.Err()
}

var (
	sshRemoteRe   = regexp.MustCompile(`[:/]([^:/]+)/([^/]+?)(?:\.git)?$`)
	httpsRemoteRe = regexp.MustCompile(`^https?://[^/]+/([^/]+)/([^/]+?)(?:\.git)?/?$`)
)

func repoFromRemote(url string) string {
	if m := sshRemoteRe.FindStringSubmatch(url); m != nil {
		return m[1] + "/" + strings.TrimSuffix(m[2], ".git")
	}
	if m := httpsRemoteRe.FindStringSubmatch(url); m != nil {
		return m[1] + "/" + strings.TrimSuffix(m[2], ".git")
	}
	return ""
}

func readGitBranch(gitPath string) (string, error) {
	head, err := os.ReadFile(filepath.Join(gitPath, "HEAD"))
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(head))
	if strings.HasPrefix(line, "ref: refs/heads/") {
		return strings.TrimPrefix(line, "ref: refs/heads/"), nil
	}
	return "", nil
}

func detectStack(root string) Stack {
	candidates := []struct {
		file  string
		stack Stack
	}{
		{"go.mod", StackGo},
		{"composer.json", StackPHP},
		{"package.json", StackNode},
		{"requirements.txt", StackPython},
		{"pyproject.toml", StackPython},
		{"Gemfile", StackRuby},
	}
	cur := root
	for {
		for _, c := range candidates {
			if _, err := os.Stat(filepath.Join(cur, c.file)); err == nil {
				return c.stack
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return StackUnknown
}

func findManifest(root string) (bool, string) {
	candidates := []string{".domain/manifest.yaml", ".domain/manifest.yml"}
	cur := root
	for {
		for _, c := range candidates {
			p := filepath.Join(cur, c)
			if _, err := os.Stat(p); err == nil {
				return true, p
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return false, ""
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, " ", "-")
	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	s = re.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	if s == "" {
		return "project"
	}
	return s
}
