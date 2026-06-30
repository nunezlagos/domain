package sources

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"nunezlagos/domain/internal/service/wizardplan"
)

// CodebaseSource recorre el repo buscando endpoints HTTP, services, types
// y otros símbolos relacionados al prompt. Versión estática (sin AST) que
// usa regex + heurística — suficiente para hits relevantes.
type CodebaseSource struct {
	ProjectRoot string
	MaxHits     int // default 10
}

func (s *CodebaseSource) Name() string { return "code" }

var (
	endpointRe = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+\s+/[^"]+)"`)
	funcRe     = regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?([A-Z][A-Za-z0-9_]+)`)
	typeRe     = regexp.MustCompile(`^type\s+([A-Z][A-Za-z0-9_]+)\s+(?:struct|interface)`)
)

func (s *CodebaseSource) Run(ctx context.Context, env *wizardplan.ContextEnvelope) error {
	root := s.ProjectRoot
	if root == "" {
		root = "."
	}
	maxHits := s.MaxHits
	if maxHits <= 0 {
		maxHits = 10
	}


	keywords := extractKeywords(env.RawPrompt, 8)
	if len(keywords) == 0 {
		return nil
	}

	hits := []wizardplan.CodeHit{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || ctx.Err() != nil {
			return walkErr
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == "node_modules" || base == "vendor" || base == ".git" ||
				base == ".claude" || base == "openspec" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if len(hits) >= maxHits {
			return filepath.SkipDir
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		rel, _ := filepath.Rel(root, path)
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		lineNum := 0
		for scanner.Scan() {
			if len(hits) >= maxHits {
				return filepath.SkipDir
			}
			lineNum++
			line := scanner.Text()
			lower := strings.ToLower(line)


			if m := endpointRe.FindStringSubmatch(line); m != nil {
				if matchesAnyKeyword(lower, keywords) {
					hits = append(hits, wizardplan.CodeHit{
						Path: rel, Line: lineNum, Symbol: m[1],
						Snippet: strings.TrimSpace(line), Category: "endpoint",
					})
					continue
				}
			}

			if m := funcRe.FindStringSubmatch(line); m != nil && matchesAnyKeyword(lower, keywords) {
				cat := "service"
				if strings.HasSuffix(rel, "handler.go") || strings.Contains(rel, "/api/handler/") {
					cat = "handler"
				}
				hits = append(hits, wizardplan.CodeHit{
					Path: rel, Line: lineNum, Symbol: m[1],
					Snippet: strings.TrimSpace(line), Category: cat,
				})
				continue
			}
			if m := typeRe.FindStringSubmatch(line); m != nil && matchesAnyKeyword(lower, keywords) {
				hits = append(hits, wizardplan.CodeHit{
					Path: rel, Line: lineNum, Symbol: m[1],
					Snippet: strings.TrimSpace(line), Category: "type",
				})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	env.Code = &wizardplan.CodeGrepFinding{Hits: hits}


	if len(hits) > 0 {
		dir := dirComponent(hits[0].Path)
		conf := 0.5
		if len(hits) >= 3 {
			conf = 0.7
		}
		env.Touch(wizardplan.SlotComponent, dir, "code", conf,
			"inferido del top hit "+hits[0].Path)
	}
	return nil
}

func extractKeywords(text string, max int) []string {
	stopwords := map[string]bool{
		"el": true, "la": true, "los": true, "las": true, "de": true, "del": true,
		"un": true, "una": true, "unos": true, "unas": true, "y": true, "o": true,
		"que": true, "qué": true, "como": true, "cómo": true, "para": true, "por": true,
		"the": true, "a": true, "and": true, "or": true, "is": true, "to": true,
		"of": true, "in": true, "on": true, "for": true, "this": true, "that": true,
		"no": true, "se": true, "es": true,
	}
	wordRe := regexp.MustCompile(`[a-záéíóúñA-ZÁÉÍÓÚÑ]{4,}`)
	seen := map[string]bool{}
	out := []string{}
	for _, m := range wordRe.FindAllString(text, -1) {
		w := strings.ToLower(m)
		if stopwords[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
		if len(out) >= max {
			break
		}
	}
	return out
}

func matchesAnyKeyword(line string, keywords []string) bool {
	for _, k := range keywords {
		if strings.Contains(line, k) {
			return true
		}
	}
	return false
}

func dirComponent(path string) string {

	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) >= 3 && parts[0] == "internal" {
		return strings.Join(parts[1:len(parts)-1], "/")
	}
	if len(parts) >= 2 {
		return strings.Join(parts[:len(parts)-1], "/")
	}
	return path
}
