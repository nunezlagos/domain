package manifest

import (
	"fmt"
	"os"
	"strings"
)

type BlockMarkerReverser struct{}

func (r *BlockMarkerReverser) CanRevert(entry Entry) bool {
	return entry.RevertStrategy == "remove_block"
}

func (r *BlockMarkerReverser) Revert(entry Entry) error {
	data, err := os.ReadFile(entry.Path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	markerOpen, _ := entry.RevertMetadata["marker_open"].(string)
	markerClose, _ := entry.RevertMetadata["marker_close"].(string)

	if markerOpen == "" || markerClose == "" {
		return fmt.Errorf("missing marker metadata")
	}

	content := string(data)
	openIdx := strings.Index(content, markerOpen)
	closeIdx := strings.Index(content, markerClose)

	if openIdx == -1 || closeIdx == -1 {
		return fmt.Errorf("markers not found in file")
	}

	closeEnd := closeIdx + len(markerClose)

	if closeEnd < len(content) && content[closeEnd] == '\n' {
		closeEnd++
	}

	beforeOpen := ""
	if openIdx > 0 && content[openIdx-1] == '\n' {
		beforeOpen = "\n"
		openIdx--
	}

	newContent := content[:openIdx] + content[closeEnd:]
	if beforeOpen != "" {
		newContent = content[:openIdx+1] + content[closeEnd:]
	}


	if strings.HasPrefix(newContent, "\n") && beforeOpen == "\n" {

	}


	for strings.Contains(newContent, "\n\n\n") {
		newContent = strings.ReplaceAll(newContent, "\n\n\n", "\n\n")
	}

	return os.WriteFile(entry.Path, []byte(newContent), 0o644)
}
