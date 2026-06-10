// HU-23.1 legacy-import — importa contenido desde formatos externos a Domain.
//
// MVP: markdown-vault (Obsidian-style). Cada archivo .md → un knowledge_doc.
// Front matter YAML → metadata. Wikilinks [[Other]] se preservan como
// references (text-only por ahora).
//
// Otros formatos (JSON dump, Notion export) quedan como futuras extensions
// con interface Importer.
package imports

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

// Importer es el interface que cada format adapter implementa.
type Importer interface {
	Format() string
	Parse(input io.Reader) ([]Document, error)
}

// Document es el shape unificado que el service de knowledge consume.
type Document struct {
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Tags     []string          `json:"tags,omitempty"`
	Source   string            `json:"source"`              // "markdown:vault.zip/notes/x.md"
	Wikilinks []string         `json:"wikilinks,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// MarkdownVaultImporter parsea un ZIP con archivos .md.
type MarkdownVaultImporter struct{}

func (MarkdownVaultImporter) Format() string { return "markdown-vault" }

// Parse acepta un ZIP reader y extrae todos los .md.
func (MarkdownVaultImporter) Parse(input io.Reader) ([]Document, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, fmt.Errorf("read zip: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	var docs []Document
	for _, f := range zr.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".md") {
			continue
		}
		if f.UncompressedSize64 > 5*1024*1024 { // skip files > 5MB
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		doc := ParseMarkdownFile(f.Name, string(body))
		docs = append(docs, doc)
	}
	if len(docs) == 0 {
		return nil, errors.New("no .md files found in archive")
	}
	return docs, nil
}

var frontMatterRe = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)
var wikilinkRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)

// ParseMarkdownFile extrae title (filename or first H1), body, tags, wikilinks.
func ParseMarkdownFile(path, content string) Document {
	doc := Document{
		Source: "markdown:" + path,
		Metadata: map[string]string{
			"file_path": path,
		},
	}

	// 1. Front matter (YAML between --- lines).
	if m := frontMatterRe.FindStringSubmatch(content); len(m) == 3 {
		fm := m[1]
		content = m[2]
		parseFrontMatter(fm, &doc)
	}

	// 2. Title: front matter > H1 > filename.
	if doc.Title == "" {
		doc.Title = extractH1(content)
	}
	if doc.Title == "" {
		doc.Title = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	// 3. Wikilinks.
	matches := wikilinkRe.FindAllStringSubmatch(content, -1)
	seen := map[string]bool{}
	for _, m := range matches {
		ref := strings.TrimSpace(m[1])
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		doc.Wikilinks = append(doc.Wikilinks, ref)
	}

	doc.Body = strings.TrimSpace(content)
	return doc
}

func parseFrontMatter(fm string, doc *Document) {
	for _, line := range strings.Split(fm, "\n") {
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"'`)
		switch key {
		case "title":
			doc.Title = val
		case "tags":
			// Soporta lista inline [a, b, c] o single string
			val = strings.Trim(val, "[]")
			for _, t := range strings.Split(val, ",") {
				t = strings.TrimSpace(strings.Trim(t, `"'`))
				if t != "" {
					doc.Tags = append(doc.Tags, t)
				}
			}
		default:
			doc.Metadata[key] = val
		}
	}
}

func extractH1(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return ""
}
