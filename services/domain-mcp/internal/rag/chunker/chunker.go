// Package chunker — issue-03.4 RAG chunking strategy.
//
// Estrategia inicial: recursive character splitting orientada a párrafos +
// frases. Target ≈ 512 tokens (estimación: 1 token ≈ 4 chars en español).
// Window por chunk: 2048 chars con overlap de 200 chars entre chunks
// consecutivos (mejora recall de chunks pegados a boundaries semánticos).
//
// La estrategia es deterministic: misma entrada → mismos chunks.
package chunker

import (
	"strings"
)

const (
	DefaultMaxChars = 2048
	DefaultOverlap  = 200


	MinChunkChars = 50
)

type Options struct {
	MaxChars int
	Overlap  int
}

func defaults(o Options) Options {
	if o.MaxChars <= 0 {
		o.MaxChars = DefaultMaxChars
	}
	if o.Overlap < 0 {
		o.Overlap = 0
	}
	if o.Overlap >= o.MaxChars {
		o.Overlap = o.MaxChars / 4
	}
	return o
}

// Chunk parte text en bloques con overlap. Primero intenta cortar en
// boundaries naturales (párrafo > newline > frase > espacio); cae a corte
// duro solo si no encuentra un boundary en el último 30% del chunk.
func Chunk(text string, opt Options) []string {
	opt = defaults(opt)
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= opt.MaxChars {
		return []string{text}
	}

	var chunks []string
	start := 0
	for start < len(text) {
		end := start + opt.MaxChars
		if end >= len(text) {
			chunks = append(chunks, strings.TrimSpace(text[start:]))
			break
		}

		lookback := start + (opt.MaxChars * 7 / 10)
		cut := findBoundary(text, lookback, end)
		if cut <= start {
			cut = end // fallback duro
		}
		chunk := strings.TrimSpace(text[start:cut])
		if len(chunk) >= MinChunkChars {
			chunks = append(chunks, chunk)
		} else if n := len(chunks); n > 0 {

			chunks[n-1] += " " + chunk
		}

		next := cut - opt.Overlap
		if next <= start {
			next = start + 1
		}
		start = next
	}
	return chunks
}

// findBoundary busca el separador natural más cercano en (lookback, end].
// Prefiere: doble newline, single newline, punto+espacio, espacio.
func findBoundary(text string, lookback, end int) int {
	if end > len(text) {
		end = len(text)
	}
	window := text[lookback:end]

	if i := strings.LastIndex(window, "\n\n"); i >= 0 {
		return lookback + i + 2
	}

	if i := strings.LastIndex(window, "\n"); i >= 0 {
		return lookback + i + 1
	}

	if i := strings.LastIndex(window, ". "); i >= 0 {
		return lookback + i + 2
	}

	if i := strings.LastIndex(window, " "); i >= 0 {
		return lookback + i + 1
	}
	return end
}
