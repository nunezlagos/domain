//go:build !linux
// +build !linux

package installer

import "fmt"

// readOSRelease es un stub en sistemas no-linux. En esos casos
// retornamos error y DetectPlatform() cae a DistroOther.
func readOSRelease() (string, string, error) {
	return "", "", fmt.Errorf("readOSRelease only available on linux")
}
