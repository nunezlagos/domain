package orchestrator

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// DOMAINSERV-244: DOMAINSERV-230 cableó el cierre del workflow en
// UpdateFlowRunStatus, pero ese NO es el único camino por el que un flow_run llega a
// un estado terminal. Hay varios writers más y ninguno cierra el workflow
// correlacionado.
//
// Hoy eso no es un bug: las filas de `workflows` nacen del hook de métricas del MCP,
// no del runner, así que una corrida que vive entera dentro del runner no tiene fila
// que quede abierta. El invariante se sostiene — pero POR UNA COINCIDENCIA DE
// ARQUITECTURA, no por diseño. El día que el runner genere su fila de workflow, esos
// caminos van a dejar workflows sin cerrar y el síntoma va a ser idéntico al que 230
// acabó de arreglar.
//
// Este guard no arregla eso: CONGELA la deuda y falla ante un writer NUEVO, que es el
// momento en que alguien tiene que decidir si ese camino necesita cerrar el workflow.
// Mismo patrón que .size-lint-baseline usa para las funciones largas: la deuda vieja
// no bloquea, lo nuevo sí.

// writersConocidos son los archivos que HOY escriben flow_runs.status, medidos el
// 2026-08-05. Agregar uno acá es una decisión deliberada, y el comentario de al lado
// tiene que decir si cierra el workflow o por qué no le hace falta.
var writersConocidos = map[string]string{
	// el único que cierra el workflow (DOMAINSERV-230)
	"internal/service/orchestrator/repository.go": "cierra el workflow vía closeWorkflowIfTerminal",
	// cancelación por pedido del usuario; hoy sin fila de workflow asociada
	"internal/service/flow/pg_repository.go": "pausa/resume/cancel del servicio de flows",
	// el runner: su corrida no produce filas en workflows (ver el header)
	"internal/runner/flow/runner.go":   "terminal del runner",
	"internal/runner/flow/claim.go":    "claim y devolución a pending",
	"internal/runner/flow/resume.go":   "resume y terminal tras resume",
	"internal/runner/flow/recovery.go": "recovery de corridas huérfanas",
	// el watchdog de heartbeats marca failed lo que dejó de latir
	"internal/scheduler/cron/system/heartbeat_watcher.go": "watchdog de heartbeat",
}

func TestFlowRuns_NingunWriterNuevoDeStatusSinDecidirElCierreDelWorkflow(t *testing.T) {
	raiz := raizDelModulo(t)
	patron := regexp.MustCompile(`UPDATE\s+flow_runs\s+(?:fr\s+)?SET[^;'"` + "`" + `]*\bstatus\s*=`)

	encontrados := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(raiz, "internal"), func(ruta string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") {
			return err
		}
		contenido, err := os.ReadFile(ruta)
		if err != nil {
			return err
		}
		if patron.Match(contenido) {
			rel, _ := filepath.Rel(raiz, ruta)
			encontrados[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("recorrer internal/: %v", err)
	}
	if len(encontrados) == 0 {
		t.Fatal("el patrón no encontró NINGÚN writer de flow_runs.status: el guard no está midiendo nada")
	}

	var nuevos []string
	for archivo := range encontrados {
		if _, conocido := writersConocidos[archivo]; !conocido {
			nuevos = append(nuevos, archivo)
		}
	}
	sort.Strings(nuevos)
	if len(nuevos) > 0 {
		t.Errorf("writers NUEVOS de flow_runs.status: %s\n"+
			"Cada camino que lleva un flow_run a estado terminal tiene que decidir si cierra el workflow "+
			"correlacionado (DOMAINSERV-230/244). Si este camino no necesita cerrarlo, agregalo a "+
			"writersConocidos con el motivo; si lo necesita, cableá el cierre.",
			strings.Join(nuevos, ", "))
	}

	// contra-prueba: si un writer conocido DESAPARECE, la lista quedó desactualizada y
	// el guard protege un mapa que ya no describe el código
	var fantasmas []string
	for archivo := range writersConocidos {
		if !encontrados[archivo] {
			fantasmas = append(fantasmas, archivo)
		}
	}
	sort.Strings(fantasmas)
	if len(fantasmas) > 0 {
		t.Errorf("writersConocidos lista archivos que ya NO escriben flow_runs.status: %s — sacalos, "+
			"o el baseline deja de describir el código y tapa un writer nuevo con un hueco viejo",
			strings.Join(fantasmas, ", "))
	}
}

// raizDelModulo sube hasta el go.mod en vez de contar `..` desde el cwd. Contar niveles
// asume que el runner posiciona el cwd en el directorio del paquete, y eso no se cumple
// siempre: el guard hermano de cmd/domain pasaba con `go test ./cmd/domain/` y fallaba
// con `go test ./...` por exactamente eso.
func raizDelModulo(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			t.Fatalf("no se encontró go.mod subiendo desde %s", dir)
		}
		dir = padre
	}
}
