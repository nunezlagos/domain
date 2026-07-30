package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func escribirFixture(t *testing.T, nombre, contenido string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, nombre), []byte(contenido), 0o644))
	return dir
}

func TestMCPResultLint_CallToolResultManual_ReportaViolacion(t *testing.T) {
	dir := escribirFixture(t, "malo.go", `package mcpserver

import "github.com/mark3labs/mcp-go/mcp"

func handler() (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("hola")}}, nil
}
`)

	violations, scanned, err := lintDir(dir)
	require.NoError(t, err)
	require.Equal(t, 1, scanned)
	require.Len(t, violations, 1)
	require.Equal(t, 6, violations[0].Line)
	require.Contains(t, violations[0].Msg, "toolResultJSON")
}

func TestMCPResultLint_HelperCanonico_NoReportaNada(t *testing.T) {
	dir := escribirFixture(t, "bueno.go", `package mcpserver

import "github.com/mark3labs/mcp-go/mcp"

func handler() (*mcp.CallToolResult, error) {
	return toolResultJSON(map[string]any{"ok": true})
}

func otro() *mcp.CallToolResult {
	return mcp.NewToolResultError("boom")
}
`)

	violations, _, err := lintDir(dir)
	require.NoError(t, err)
	require.Empty(t, violations)
}

// los tests necesitan fabricar resultados arbitrarios para ejercitar el guard de bytes
func TestMCPResultLint_ArchivoDeTest_SeIgnora(t *testing.T) {
	dir := escribirFixture(t, "algo_test.go", `package mcpserver

import "github.com/mark3labs/mcp-go/mcp"

func fabricar() *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("x")}}
}
`)

	violations, scanned, err := lintDir(dir)
	require.NoError(t, err)
	require.Equal(t, 0, scanned)
	require.Empty(t, violations)
}

func TestMCPResultLint_StructHomonimoDeOtroPaquete_NoReportaFalsoPositivo(t *testing.T) {
	dir := escribirFixture(t, "otro.go", `package mcpserver

type CallToolResult struct{ Content string }

func propio() otropkg.CallToolResult {
	return otropkg.CallToolResult{Content: "no es del paquete mcp"}
}
`)

	violations, _, err := lintDir(dir)
	require.NoError(t, err)
	require.Empty(t, violations, "solo mcp.CallToolResult cuenta, no un homónimo de otro paquete")
}

func TestMCPResultLint_DirInexistente_DevuelveError(t *testing.T) {
	_, _, err := lintDir(filepath.Join(t.TempDir(), "no-existe"))
	require.Error(t, err, "un error de setup es un fallo, nunca un scan vacío que pasa")
}

// el guard tiene que arrancar sin deuda tolerada: si este test falla, alguien volvió a
// armar el resultado a mano en el paquete real
func TestMCPResultLint_RepoActual_CeroViolaciones(t *testing.T) {
	violations, scanned, err := lintDir("../../internal/mcp/server")
	require.NoError(t, err)
	require.Greater(t, scanned, 20, "el scan tiene que haber leído el paquete real")
	require.Empty(t, violations, "violations: %v", violations)
}

func TestMCPResultLint_EscapeHatchSinRazon_SigueSiendoViolacion(t *testing.T) {
	dir := escribirFixture(t, "hatch_mudo.go", `package mcpserver

import "github.com/mark3labs/mcp-go/mcp"

func handler() *mcp.CallToolResult {
	// mcp-result-lint:allow
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("x")}}
}
`)

	violations, _, err := lintDir(dir)
	require.NoError(t, err)
	require.Len(t, violations, 1, "un allow sin razón es deuda sin dueño: no debe eximir")
}

func TestMCPResultLint_EscapeHatchConRazon_Exime(t *testing.T) {
	dir := escribirFixture(t, "hatch_ok.go", `package mcpserver

import "github.com/mark3labs/mcp-go/mcp"

func handler() *mcp.CallToolResult {
	// mcp-result-lint:allow este ES el helper canónico
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("x")}}
}
`)

	violations, _, err := lintDir(dir)
	require.NoError(t, err)
	require.Empty(t, violations)
}
