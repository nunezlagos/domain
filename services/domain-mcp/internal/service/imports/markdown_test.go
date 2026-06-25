package imports_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/imports"
)

func TestParseMarkdownFile_TitleFromH1(t *testing.T) {
	content := "# Mi título\n\nBody content"
	doc := imports.ParseMarkdownFile("notes/file.md", content)
	require.Equal(t, "Mi título", doc.Title)
	require.Contains(t, doc.Body, "Body content")
}

func TestParseMarkdownFile_TitleFromFilename(t *testing.T) {
	doc := imports.ParseMarkdownFile("notes/auth-design.md", "no heading here\njust body")
	require.Equal(t, "auth-design", doc.Title)
}

func TestParseMarkdownFile_FrontMatter(t *testing.T) {
	content := `---
title: Mi Doc
tags: [feature, auth, security]
status: published
---

# Body H1

Real content.`
	doc := imports.ParseMarkdownFile("x.md", content)
	require.Equal(t, "Mi Doc", doc.Title) // front matter gana sobre H1
	require.Equal(t, []string{"feature", "auth", "security"}, doc.Tags)
	require.Equal(t, "published", doc.Metadata["status"])
}

func TestParseMarkdownFile_Wikilinks(t *testing.T) {
	content := `Esto referencia a [[Other Note]] y también [[Project X|Project]] y duplicado [[Other Note]].`
	doc := imports.ParseMarkdownFile("x.md", content)
	require.Len(t, doc.Wikilinks, 2)
	require.Contains(t, doc.Wikilinks, "Other Note")
	require.Contains(t, doc.Wikilinks, "Project X")
}

// Sabotaje: front matter sin cierre `---` no debe consumir el body.
func TestSabotage_UnclosedFrontMatter(t *testing.T) {
	content := `---
title: never-closed
key: value

This should be body, not front matter.`
	doc := imports.ParseMarkdownFile("x.md", content)
	require.Empty(t, doc.Tags)

	require.Equal(t, "x", doc.Title)
}
