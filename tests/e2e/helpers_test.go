//go:build integration

package e2e_test

import "os"

// osMkdirAll wrapper para inject testable behavior si necesario.
func osMkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

func osWriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
