// Package llm — abstracción de proveedores LLM (embeddings, completion).
//
// Para MVP: solo Embedder. Completion vendrá con HU-12.x.
//
// Implementaciones:
//   - NopEmbedder: vector zero (útil cuando no hay API key configurada;
//     búsqueda degrada a tsvector-only)
//   - FakeEmbedder: vector determinístico hash-based (útil para tests
//     reproducibles sin red)
//   - OpenAIEmbedder, AnthropicEmbedder: pending HU-12.2/3
package llm

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// Embedder genera vectors de tamaño Dimensions(). El tamaño es fijo por
// implementación (no se cambia after-the-fact, las migrations dependen de él).
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}

// NopEmbedder devuelve vector zero. La búsqueda híbrida degrada cleanly al
// vector zero porque cosine(v, 0) está undefined; el service filtra vector zero
// y queda con tsvector-only ranking.
type NopEmbedder struct {
	Dim int
}

func (n NopEmbedder) Dimensions() int {
	if n.Dim == 0 {
		return 1536 // default to match migration 000006 vector(1536)
	}
	return n.Dim
}

func (n NopEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, n.Dimensions()), nil
}

func (n NopEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	dim := n.Dimensions()
	for i := range out {
		out[i] = make([]float32, dim)
	}
	return out, nil
}

// FakeEmbedder genera un vector determinístico desde sha256(text). Útil para
// integration tests: la misma frase devuelve siempre el mismo embedding, y
// dos frases distintas producen vectors distantes. NO usar en producción.
type FakeEmbedder struct {
	Dim int
}

func (f FakeEmbedder) Dimensions() int {
	if f.Dim == 0 {
		return 1536
	}
	return f.Dim
}

func (f FakeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	dim := f.Dimensions()
	out := make([]float32, dim)
	// Seed: sha256 del text. Replicamos el hash hasta llenar dim*4 bytes.
	seed := sha256.Sum256([]byte(text))
	var sumSq float64
	for i := 0; i < dim; i++ {
		idx := (i * 4) % len(seed)
		bits := binary.BigEndian.Uint32(append(seed[idx:], seed[:idx]...)[:4])
		// Map a [-1, 1]
		v := float32(int32(bits)) / float32(math.MaxInt32)
		out[i] = v
		sumSq += float64(v) * float64(v)
	}
	// Normalizar a unit-norm (mejor para cosine)
	if sumSq > 0 {
		norm := float32(math.Sqrt(sumSq))
		for i := range out {
			out[i] /= norm
		}
	}
	return out, nil
}

func (f FakeEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := f.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// TruncateText limita el texto a maxTokens tokens aproximados.
// Cuenta tokens como palabras (~4 chars/token para texto en español/inglés).
// Útil para controlar costos antes de enviar a Embed (HU-06.5).
func TruncateText(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	// Estimación rough: ~4 chars por token
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	// Truncar en límite de palabra para no cortar a mitad
	truncated := text[:maxChars]
	if idx := lastSpace(truncated); idx > maxChars/2 {
		truncated = truncated[:idx]
	}
	return truncated
}

func lastSpace(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ' ' {
			return i
		}
	}
	return -1
}

// IsZero retorna true si el vector es todo cero (NopEmbedder fingerprint).
func IsZero(v []float32) bool {
	for _, x := range v {
		if x != 0 {
			return false
		}
	}
	return true
}
