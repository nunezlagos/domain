// issue-09.6 — helpers de output gzip (de-007), idempotency key (de-008),
// replay_safe (de-009).
package flowrunner

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"
)

// CompressOutput gzip comprime el output JSON de un step.
// Retorna el slice comprimido y el tamaño original.
func CompressOutput(output any) (compressed []byte, originalSize int, err error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal output: %w", err)
	}
	originalSize = len(raw)

	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, originalSize, fmt.Errorf("gzip writer: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return nil, originalSize, fmt.Errorf("gzip write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, originalSize, fmt.Errorf("gzip close: %w", err)
	}
	return buf.Bytes(), originalSize, nil
}

// DecompressOutput descomprime un blob gzip.
func DecompressOutput(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}
	return out, nil
}

// StepIDempotencyKey genera la key idempotente para un step.
// Formato: "flow_run:<runID>:step:<stepKey>"
func StepIDempotencyKey(flowRunID uuid.UUID, stepKey string) string {
	return fmt.Sprintf("flow_run:%s:step:%s", flowRunID.String(), stepKey)
}

// ShouldSpillToS3 retorna true si el output comprimido supera 10MB.
const S3SpillThreshold = 10 * 1024 * 1024 // 10MB

func ShouldSpillToS3(compressedSize int) bool {
	return compressedSize > S3SpillThreshold
}

// IsReplaySafe returns false only if replay_safe is explicitly false.
func IsReplaySafe(stepReplaySafe *bool) bool {
	if stepReplaySafe == nil {
		return true
	}
	return *stepReplaySafe
}
