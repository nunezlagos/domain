//go:build linux


package installer

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// readOSRelease lee /etc/os-release y retorna (ID, VERSION_ID, err).
// Build tag: solo se compila en linux. En otros OS la implementacion
// del archivo os.go retorna error.
func readOSRelease() (string, string, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", "", fmt.Errorf("open /etc/os-release: %w", err)
	}
	defer f.Close()

	id := ""
	versionID := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
		switch key {
		case "ID":
			id = val
		case "VERSION_ID":
			versionID = val
		}
	}
	if err := scanner.Err(); err != nil {
		return id, versionID, err
	}
	if id == "" {
		return "", "", fmt.Errorf("ID not found in /etc/os-release")
	}
	return id, versionID, nil
}
