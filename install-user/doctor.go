package main

import "fmt"

// runDoctor valida que la instalación del cliente domain esté completa y
// consistente bajo HOME, y reporta cada chequeo con ✓ (ok) o ✗ (falla).
// Los checks individuales viven en doctor_hooks.go, doctor_mcp.go y
// doctor_config.go (mismo package). Devuelve el exit code: 0 si TODO lo
// crítico pasó, !=0 si falta algo crítico. La salud del MCP NO es crítica.
func runDoctor(home string) int {
	step("domain doctor — self-check")

	critical := 0
	critical += checkPython3()
	critical += checkHooks(home)
	critical += checkHookMatchers(home)
	critical += checkMCPEntry(home)
	critical += checkPermissions(home)
	critical += checkInstructions(home)
	critical += checkClaudeMdExcludes(home)
	critical += checkOpencodePermission(home)
	critical += checkOpencodePlugin(home)
	checkMCPHealth(home) // best-effort, no suma a critical

	step("Resumen")
	if critical == 0 {
		ok("instalación consistente — todos los chequeos críticos pasaron")
		return 0
	}
	failL(fmt.Sprintf("%d chequeo(s) crítico(s) fallaron — re-corré el install para reparar", critical))
	return 1
}

// doctorStringSet convierte un array JSON de strings en un set para lookup.
// Tolera tipos mixtos (ignora los no-string) y valores ausentes.
func doctorStringSet(v any) map[string]bool {
	set := map[string]bool{}
	arr, ok := v.([]any)
	if !ok {
		return set
	}
	for _, e := range arr {
		if s, ok := e.(string); ok {
			set[s] = true
		}
	}
	return set
}
