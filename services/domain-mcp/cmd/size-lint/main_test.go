package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func bigFunc(name string, bodyLines int) string {
	var b strings.Builder
	b.WriteString("func " + name + "() {\n")
	for i := 0; i < bodyLines; i++ {
		b.WriteString("\t_ = " + itoa(i) + "\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var d []byte
	for i > 0 {
		d = append([]byte{byte('0' + i%10)}, d...)
		i /= 10
	}
	return string(d)
}

func writeGo(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package x\n\n"+body), 0o644))
}

func TestScan_FuncOverMax_Detectada(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, dir, "big.go", bigFunc("Grande", 60))
	v := scan(dir, 50)
	require.Len(t, v, 1)
	require.Equal(t, "Grande", v[0].fn)
	require.Greater(t, v[0].lines, 50)
}

func TestScan_FuncBajoMax_NoDetectada(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, dir, "small.go", bigFunc("Chica", 10))
	require.Empty(t, scan(dir, 50))
}

func TestScan_EscapeHatch_Exenta(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, dir, "hatch.go", "// size-lint:allow generado, inherentemente largo\n"+bigFunc("Grande", 60))
	require.Empty(t, scan(dir, 50), "size-lint:allow exime la función")
}

func TestScan_TestFile_Exento(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, dir, "big_test.go", bigFunc("GrandeTest", 60))
	require.Empty(t, scan(dir, 50), "*_test.go está exento")
}

func TestScan_MainGo_Exento(t *testing.T) {
	dir := t.TempDir()
	writeGo(t, dir, "main.go", bigFunc("Grande", 60))
	require.Empty(t, scan(dir, 50), "main.go (wiring) está exento")
}
