package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installOpencodeGlobalInstruction es idempotente: la 2a corrida no debe
// crear un backup nuevo de opencode.json (no muta nada).
func TestInstallOpencodeGlobalInstruction_Idempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	paths := Platform{OS: "linux"}.Paths()

	// Simular opencode instalado con un opencode.json del usuario.
	if err := os.MkdirAll(paths.OpencodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.OpencodeMCP, []byte(`{"instructions":["AGENTS.md"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installOpencodeGlobalInstruction(paths, "20260101T000000Z"); err != nil {
		t.Fatalf("1a corrida: %v", err)
	}
	backups1, _ := filepath.Glob(paths.OpencodeMCP + ".backup-*")
	if len(backups1) != 1 {
		t.Fatalf("1a corrida: esperaba 1 backup, hay %d", len(backups1))
	}

	// 2a corrida con distinto timestamp: no debe agregar otro backup.
	if err := installOpencodeGlobalInstruction(paths, "20260202T000000Z"); err != nil {
		t.Fatalf("2a corrida: %v", err)
	}
	backups2, _ := filepath.Glob(paths.OpencodeMCP + ".backup-*")
	if len(backups2) != 1 {
		t.Errorf("2a corrida no idempotente: %d backups, want 1: %v", len(backups2), backups2)
	}

	raw, _ := os.ReadFile(paths.OpencodeMCP)
	if !strings.Contains(string(raw), "AGENTS.md") {
		t.Error("se perdió la instruction del usuario 'AGENTS.md'")
	}
	if !strings.Contains(string(raw), "instructions/domain.md") {
		t.Error("no se agregó 'instructions/domain.md'")
	}
}

// DOMAINSERV-101: installGlobalInstructions ya NO escribe la instruction de
// OpenCode. Esa responsabilidad se movió al cluster post-Apply (main.go) para
// que ~/.config/opencode ya exista cuando se escriba. Aunque el dir esté
// presente, installGlobalInstructions no debe tocarlo: el fix del ordering del
// install fresco depende de este desacople.
func TestInstallGlobalInstructions_DoesNotWriteOpencodeInstruction(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	paths := Platform{OS: "linux"}.Paths()

	// OpenCode ya presente (dir + json): el código viejo escribiría
	// instructions/domain.md acá; el nuevo delega al paso post-Apply.
	if err := os.MkdirAll(paths.OpencodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.OpencodeMCP, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installGlobalInstructions(home, "20260101T000000Z"); err != nil {
		t.Fatalf("install: %v", err)
	}

	instrPath := filepath.Join(paths.OpencodeDir, "instructions", "domain.md")
	if _, err := os.Stat(instrPath); !os.IsNotExist(err) {
		t.Fatalf("installGlobalInstructions no debe escribir la instruction de OpenCode (DOMAINSERV-101); apareció %s", instrPath)
	}
}

// installGlobalInstructions escribe el cuerpo real en ~/.claude/domain.md y deja
// SOLO un @import en ~/.claude/CLAUDE.md (no el cuerpo completo). issue-54.1.
func TestInstallGlobalInstructions_WritesDomainMdAndImport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := installGlobalInstructions(home, "20260101T000000Z"); err != nil {
		t.Fatalf("install: %v", err)
	}

	dm, err := os.ReadFile(claudeDomainMdPath(home))
	if err != nil {
		t.Fatalf("read domain.md: %v", err)
	}
	if len(dm) == 0 {
		t.Fatal("domain.md quedó vacío")
	}

	cm, err := os.ReadFile(claudeGlobalPath(home))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(cm), "@domain.md") {
		t.Fatal("CLAUDE.md no tiene el @import a domain.md")
	}
	// El cuerpo real NO debe duplicarse dentro de CLAUDE.md: solo el import.
	if strings.Contains(string(cm), "@domain.md\n") && len(cm) > len(dm) {
		t.Fatal("CLAUDE.md parece tener el cuerpo completo en vez de solo el import")
	}

	// Idempotente: segunda corrida no crea backups nuevos.
	if err := installGlobalInstructions(home, "20260202T000000Z"); err != nil {
		t.Fatalf("2a corrida: %v", err)
	}
	bk, _ := filepath.Glob(claudeDomainMdPath(home) + ".backup-*")
	if len(bk) != 0 {
		t.Fatalf("corrida idempotente no debe backupear domain.md, hay %v", bk)
	}
}

// La persona vive en ~/.claude/persona.md (editable) y domain.md la referencia
// con @persona.md, no inline. Idempotente.
func TestInstallGlobalInstructions_WritesPersonaAndReference(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := installGlobalInstructions(home, "20260101T000000Z"); err != nil {
		t.Fatalf("install: %v", err)
	}

	pm, err := os.ReadFile(claudePersonaMdPath(home))
	if err != nil {
		t.Fatalf("read persona.md: %v", err)
	}
	if len(pm) == 0 {
		t.Fatal("persona.md quedó vacío")
	}

	dm, err := os.ReadFile(claudeDomainMdPath(home))
	if err != nil {
		t.Fatalf("read domain.md: %v", err)
	}
	if !strings.Contains(string(dm), "@persona.md") {
		t.Fatal("domain.md no referencia @persona.md")
	}

	// Idempotente: segunda corrida no crea backups nuevos de persona.md.
	if err := installGlobalInstructions(home, "20260202T000000Z"); err != nil {
		t.Fatalf("2a corrida: %v", err)
	}
	if bk, _ := filepath.Glob(claudePersonaMdPath(home) + ".backup-*"); len(bk) != 0 {
		t.Fatalf("corrida idempotente no debe backupear persona.md, hay %v", bk)
	}
}

// upsertDomainBlock en contenido vacío: escribe solo el bloque.
func TestUpsertDomainBlock_InsertIntoEmpty(t *testing.T) {
	out := upsertDomainBlock("")
	if !strings.Contains(out, domainBlockStart) || !strings.Contains(out, domainBlockEnd) {
		t.Fatal("falta el par de marcadores domain")
	}
	if !hasUpToDateDomainBlock(out) {
		t.Fatal("el bloque insertado debería reconocerse como up-to-date")
	}
	if strings.Count(out, domainBlockStart) != 1 {
		t.Fatalf("se esperaba 1 marcador start, hay %d", strings.Count(out, domainBlockStart))
	}
}

// upsertDomainBlock preserva el contenido del usuario fuera de los marcadores
// y agrega el bloque al final.
func TestUpsertDomainBlock_PreservesUserContent(t *testing.T) {
	user := "# Mi CLAUDE.md\n\n- regla propia del usuario\n"
	out := upsertDomainBlock(user)
	if !strings.Contains(out, "regla propia del usuario") {
		t.Fatal("se perdió contenido del usuario")
	}
	if !strings.Contains(out, domainBlockStart) {
		t.Fatal("no se agregó el bloque domain")
	}
	// El bloque domain debe quedar DESPUÉS del contenido del usuario.
	if strings.Index(out, "regla propia") > strings.Index(out, domainBlockStart) {
		t.Fatal("el bloque domain debería ir al final, tras el contenido del usuario")
	}
}

// upsertDomainBlock reemplaza un bloque viejo sin duplicar y preserva lo de
// afuera (antes y después del bloque).
func TestUpsertDomainBlock_ReplacesOldBlockNoDuplicate(t *testing.T) {
	old := "# Header del usuario\n\n" +
		domainBlockStart + "\nCONTENIDO VIEJO DE DOMAIN\n" + domainBlockEnd +
		"\n\n# Footer del usuario\n"
	out := upsertDomainBlock(old)

	if strings.Contains(out, "CONTENIDO VIEJO DE DOMAIN") {
		t.Fatal("el contenido viejo del bloque no fue reemplazado")
	}
	if strings.Count(out, domainBlockStart) != 1 {
		t.Fatalf("se duplicó el marcador start: %d", strings.Count(out, domainBlockStart))
	}
	if strings.Count(out, domainBlockEnd) != 1 {
		t.Fatalf("se duplicó el marcador end: %d", strings.Count(out, domainBlockEnd))
	}
	if !strings.Contains(out, "# Header del usuario") || !strings.Contains(out, "# Footer del usuario") {
		t.Fatal("se perdió contenido del usuario alrededor del bloque")
	}
	if !hasUpToDateDomainBlock(out) {
		t.Fatal("el bloque reemplazado debería ser up-to-date")
	}
}

// upsertDomainBlock es idempotente: aplicarlo dos veces da el mismo resultado.
func TestUpsertDomainBlock_Idempotent(t *testing.T) {
	user := "# Usuario\n\ntexto\n"
	once := upsertDomainBlock(user)
	twice := upsertDomainBlock(once)
	if once != twice {
		t.Fatalf("no es idempotente:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
	if strings.Count(twice, domainBlockStart) != 1 {
		t.Fatalf("idempotencia rota: %d marcadores start", strings.Count(twice, domainBlockStart))
	}
}

// upsertStringInArray: agrega si falta, no duplica, preserva existentes.
func TestUpsertStringInArray(t *testing.T) {
	m := map[string]any{"instructions": []any{"AGENTS.md"}}

	if !upsertStringInArray(m, "instructions", "instructions/domain.md") {
		t.Fatal("debería haber modificado el map agregando la nueva entry")
	}
	arr := m["instructions"].([]any)
	if len(arr) != 2 {
		t.Fatalf("se esperaban 2 entradas, hay %d", len(arr))
	}

	// Segunda aplicación: no duplica.
	if upsertStringInArray(m, "instructions", "instructions/domain.md") {
		t.Fatal("no debería modificar si la entry ya existe")
	}
	if len(m["instructions"].([]any)) != 2 {
		t.Fatal("se duplicó una entry existente")
	}

	// Preserva la entry del usuario.
	found := false
	for _, e := range m["instructions"].([]any) {
		if e == "AGENTS.md" {
			found = true
		}
	}
	if !found {
		t.Fatal("se perdió la entry del usuario 'AGENTS.md'")
	}
}
