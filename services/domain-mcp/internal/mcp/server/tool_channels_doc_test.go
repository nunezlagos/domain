package mcpserver

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// issue-54.1: TOOL_CHANNELS.md se declara "generado" pero no hay generador y por
// eso se desincronizó del map toolChannel. Este test es la salvaguarda: falla si
// el doc no refleja el map (tool faltante, tool en el canal equivocado, o tool en
// el doc que no está en el map). Bidireccional.

var (
	docSectionRe = regexp.MustCompile("(?m)^## ([a-z-]+) \\((\\d+)\\)\\s*$")
	docToolRe    = regexp.MustCompile("(?m)^- `(domain_[a-z_]+)`")
)

// docSection: una sección del doc con su canal, el conteo declarado en el
// header (N) y las tools realmente listadas.
type docSection struct {
	channel     string
	headerCount int
	toolsListed []string
}

// parseDocSections devuelve las secciones del doc con su conteo de header y
// tools listadas, para validar que (N) coincide con lo listado (R3).
func parseDocSections(t *testing.T) []docSection {
	t.Helper()
	doc := string(mustRead(t))
	idx := docSectionRe.FindAllStringSubmatchIndex(doc, -1)
	var out []docSection
	for i, s := range idx {
		channel := doc[s[2]:s[3]]
		count := 0
		if _, err := fmt.Sscanf(doc[s[4]:s[5]], "%d", &count); err != nil {
			t.Fatalf("conteo de header no numérico en sección %q", channel)
		}
		start := s[1]
		end := len(doc)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		var tools []string
		for _, m := range docToolRe.FindAllStringSubmatch(doc[start:end], -1) {
			tools = append(tools, m[1])
		}
		out = append(out, docSection{channel: channel, headerCount: count, toolsListed: tools})
	}
	return out
}

// parseDocChannels lee TOOL_CHANNELS.md y devuelve tool -> canal según el doc.
func parseDocChannels(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("TOOL_CHANNELS.md")
	if err != nil {
		t.Fatalf("no se pudo leer TOOL_CHANNELS.md: %v", err)
	}
	doc := string(raw)
	secs := docSectionRe.FindAllStringSubmatchIndex(doc, -1)
	out := map[string]string{}
	for i, s := range secs {
		channel := doc[s[2]:s[3]]
		start := s[1]
		end := len(doc)
		if i+1 < len(secs) {
			end = secs[i+1][0]
		}
		for _, m := range docToolRe.FindAllStringSubmatch(doc[start:end], -1) {
			out[m[1]] = channel
		}
	}
	return out
}

func TestToolChannelsDocInSync(t *testing.T) {
	t.Parallel()
	docChannels := parseDocChannels(t)

	// map -> doc: toda tool del map debe estar en el doc, en su canal correcto.
	for tool, ch := range toolChannel {
		docCh, ok := docChannels[tool]
		if !ok {
			t.Errorf("tool %q está en el map (canal %q) pero falta en TOOL_CHANNELS.md", tool, ch)
			continue
		}
		if docCh != string(ch) {
			t.Errorf("tool %q: doc dice canal %q, map dice %q", tool, docCh, ch)
		}
	}

	// doc -> map: ninguna tool del doc debe estar fuera del map.
	for tool := range docChannels {
		if _, ok := toolChannel[tool]; !ok {
			t.Errorf("tool %q está en TOOL_CHANNELS.md pero no en el map toolChannel", tool)
		}
	}

	if strings.Count(string(mustRead(t)), "domain_") == 0 {
		t.Fatal("TOOL_CHANNELS.md no lista ninguna tool — parser roto o doc vacío")
	}
}

func mustRead(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("TOOL_CHANNELS.md")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return raw
}

// issue-54.1 R3: el conteo (N) del header de cada sección debe coincidir con la
// cantidad de tools listadas debajo. El spec exige que el test falle si "los
// conteos del header no cuadran" (hallazgo del panel adversarial).
func TestToolChannelsDocHeaderCounts(t *testing.T) {
	t.Parallel()
	for _, s := range parseDocSections(t) {
		if s.headerCount != len(s.toolsListed) {
			t.Errorf("sección %q: header declara (%d) pero lista %d tools",
				s.channel, s.headerCount, len(s.toolsListed))
		}
	}
}
