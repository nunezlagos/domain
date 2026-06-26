package claudehook

import "testing"

// sessionStartCmds extrae los commands de los hooks SessionStart del doc.
func sessionStartCmds(doc map[string]any) []string {
	var out []string
	for _, h := range getSessionStart(doc) {
		if cmd, ok := h["command"].(string); ok {
			out = append(out, cmd)
		}
	}
	return out
}

func TestAddDomainHook_AgregaCuandoNoExiste(t *testing.T) {
	doc := AddDomainHook(map[string]any{})
	cmds := sessionStartCmds(doc)
	if len(cmds) != 1 || cmds[0] != domainHookCommand {
		t.Fatalf("esperaba 1 hook con comando actual, got %v", cmds)
	}
	if !DomainHookUpToDate(doc) {
		t.Fatal("DomainHookUpToDate debería ser true tras agregar")
	}
}

func TestAddDomainHook_UpgradeViejoSinDuplicar(t *testing.T) {
	// Hook viejo (sin --session-context) ya presente.
	old := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"type": "command", "command": `domain setup auto-detect "$PWD" --quiet`},
			},
		},
	}
	if DomainHookUpToDate(old) {
		t.Fatal("el comando viejo no debería contar como up-to-date")
	}
	if !HasDomainHook(old) {
		t.Fatal("HasDomainHook debería detectar el hook viejo")
	}

	upgraded := AddDomainHook(old)
	cmds := sessionStartCmds(upgraded)
	if len(cmds) != 1 {
		t.Fatalf("upgrade no debe duplicar: esperaba 1 hook, got %d (%v)", len(cmds), cmds)
	}
	if cmds[0] != domainHookCommand {
		t.Fatalf("comando no actualizado: got %q", cmds[0])
	}
	if !DomainHookUpToDate(upgraded) {
		t.Fatal("tras upgrade debería ser up-to-date")
	}
}

func TestAddDomainHook_PreservaOtrosHooks(t *testing.T) {
	doc := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"type": "command", "command": "otra-cosa --foo"},
			},
		},
	}
	out := AddDomainHook(doc)
	cmds := sessionStartCmds(out)
	if len(cmds) != 2 {
		t.Fatalf("debería preservar el hook ajeno y agregar el de domain: got %v", cmds)
	}
	hasOther, hasDomain := false, false
	for _, c := range cmds {
		if c == "otra-cosa --foo" {
			hasOther = true
		}
		if c == domainHookCommand {
			hasDomain = true
		}
	}
	if !hasOther || !hasDomain {
		t.Fatalf("falta alguno: other=%v domain=%v", hasOther, hasDomain)
	}
}
