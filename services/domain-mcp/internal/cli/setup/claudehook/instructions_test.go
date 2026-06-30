package claudehook

import (
	"strings"
	"testing"

	"nunezlagos/domain/internal/agentprotocol"
)

func TestUpsertDomainBlock_EmptyContent(t *testing.T) {
	out := upsertDomainBlock("")
	if !strings.Contains(out, domainBlockStart) || !strings.Contains(out, domainBlockEnd) {
		t.Fatal("bloque domain no insertado en contenido vacío")
	}
	if !strings.Contains(out, agentprotocol.Stub) {
		t.Fatal("Stub no presente en el bloque")
	}
	if !HasUpToDateDomainBlock(out) {
		t.Fatal("HasUpToDateDomainBlock debería ser true tras insertar")
	}
}

func TestUpsertDomainBlock_PreservesUserContent(t *testing.T) {
	user := "# Mi CLAUDE.md\n\nReglas personales que NO se deben tocar.\n"
	out := upsertDomainBlock(user)
	if !strings.Contains(out, "Reglas personales que NO se deben tocar.") {
		t.Fatal("contenido del usuario se perdió")
	}
	if !strings.Contains(out, domainBlockStart) {
		t.Fatal("bloque domain no agregado")
	}
	// El contenido del usuario va primero; el bloque domain al final.
	if strings.Index(out, "Reglas personales") > strings.Index(out, domainBlockStart) {
		t.Fatal("el bloque domain debería ir DESPUÉS del contenido del usuario")
	}
}

func TestUpsertDomainBlock_ReplacesOldBlockNoDuplicate(t *testing.T) {
	user := "# Mío\n\ntexto\n"
	stale := user + "\n" + domainBlockStart + "\nVERSION VIEJA DEL STUB\n" + domainBlockEnd + "\n"
	out := upsertDomainBlock(stale)

	if strings.Count(out, domainBlockStart) != 1 {
		t.Fatalf("se esperaba 1 marcador start, hay %d (duplicado)", strings.Count(out, domainBlockStart))
	}
	if strings.Contains(out, "VERSION VIEJA DEL STUB") {
		t.Fatal("el bloque viejo no se reemplazó")
	}
	if !strings.Contains(out, agentprotocol.Stub) {
		t.Fatal("el Stub actual no quedó en el bloque")
	}
	if !strings.Contains(out, "texto") {
		t.Fatal("contenido del usuario se perdió en el reemplazo")
	}
}

func TestUpsertDomainBlock_Idempotent(t *testing.T) {
	once := upsertDomainBlock("# user\n")
	twice := upsertDomainBlock(once)
	if once != twice {
		t.Fatal("upsert no idempotente: una segunda pasada cambió el contenido")
	}
	if strings.Count(twice, domainBlockStart) != 1 {
		t.Fatal("upsert idempotente debe mantener un solo bloque")
	}
}

func TestHasUpToDateDomainBlock_StaleReturnsFalse(t *testing.T) {
	stale := domainBlockStart + "\nalgo viejo\n" + domainBlockEnd
	if HasUpToDateDomainBlock(stale) {
		t.Fatal("un bloque con contenido viejo no debería contar como up-to-date")
	}
}
