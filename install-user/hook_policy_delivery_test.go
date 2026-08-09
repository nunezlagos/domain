package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DOMAINSERV-257: las policies vivían en la BD y NADA las hacía llegar al camino donde se
// violan. El único disparador estaba en sdd-review —dentro de un flow que LLEGA a esa
// fase— y aun ahí entregaba la primera línea truncada a 160 chars, no la regla. Medido: 8
// commits directos a main después de abrir el ticket, con ramas-y-tags v3 activa y sus
// 4636 chars de cuerpo sin entregar nunca.
//
// La entrega va por permissionDecisionReason y no por additionalContext: el reason es
// campo confirmado de PreToolUse. Estos tests miden el JSON que EMITE el hook, que es lo
// verificable desde acá; que el cliente lo muestre es la otra mitad, y se verifica a mano.

const centinelaPolicy = "Nada se commitea directo a main"

// escribirCachePolicies deja el body donde el pre-edit lo busca: el nombre del directorio
// es el basename del toplevel git, la misma derivación que hacen los dos hooks. Si
// divergieran, el pre-edit leería un directorio que nadie escribe y la entrega no
// ocurriría NUNCA, sin error — por eso el test usa la ruta real y no una inyectada.
func escribirCachePolicies(t *testing.T, home, repoDir, contenido string) {
	t.Helper()
	dir := filepath.Join(home, ".local", "state", "domain", "policy-bodies", filepath.Base(repoDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "git_workflow.md"), []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}

func decisionDe(t *testing.T, salida string) (string, string) {
	t.Helper()
	var v struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
			Reason             string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if strings.TrimSpace(salida) == "" {
		return "", ""
	}
	if err := json.Unmarshal([]byte(salida), &v); err != nil {
		t.Fatalf("la salida del hook no es UN solo JSON: %v\nsalida: %q", err, salida)
	}
	return v.HookSpecificOutput.PermissionDecision, v.HookSpecificOutput.Reason
}

func TestPolicyDelivery_ConCacheElCommitEntregaElBody(t *testing.T) {
	home := t.TempDir()
	repo := repoGitDePrueba(t)
	escribirCachePolicies(t, home, repo, "== policy ramas-y-tags (v3) ==\n"+centinelaPolicy)

	salida := correrHookEnRepo(t, "domain-pre-edit.sh", home, repo, payloadIntentoDeCommit(t))

	decision, reason := decisionDe(t, salida)
	if decision != "ask" {
		t.Fatalf("esperaba decision 'ask' para garantizar la entrega, obtuve %q (salida: %q)", decision, salida)
	}
	if !strings.Contains(reason, centinelaPolicy) {
		t.Fatalf("el reason no trae el CUERPO de la policy, que es justo lo que el índice no entrega.\nreason: %q", reason)
	}
}

// El invariante de costo (DOMAINSERV-177): se paga una interrupción por sesión, no una por
// commit. Un gate que molesta en cada commit se termina desactivando.
func TestPolicyDelivery_SegundoCommitDeLaMismaSesion_NoRepiteLaEntrega(t *testing.T) {
	home := t.TempDir()
	repo := repoGitDePrueba(t)
	escribirCachePolicies(t, home, repo, centinelaPolicy)

	correrHookEnRepo(t, "domain-pre-edit.sh", home, repo, payloadIntentoDeCommit(t))
	salida := correrHookEnRepo(t, "domain-pre-edit.sh", home, repo, payloadIntentoDeCommit(t))

	_, reason := decisionDe(t, salida)
	if strings.Contains(reason, centinelaPolicy) {
		t.Fatalf("la entrega se repitió en el segundo commit de la misma sesión: %q", reason)
	}
}

// Fail-OPEN deliberado: sin cache (VPS caído en el SessionStart) el commit sigue su curso.
// Este es el guard contra convertir la entrega en un deny accidental — un gate
// insatisfacible empuja al bypass permanente, que es DOMAINSERV-111/175/195.
func TestPolicyDelivery_SinCache_NoEntregaNiBloquea(t *testing.T) {
	home := t.TempDir()
	repo := repoGitDePrueba(t)

	salida := correrHookEnRepo(t, "domain-pre-edit.sh", home, repo, payloadIntentoDeCommit(t))

	_, reason := decisionDe(t, salida)
	if strings.Contains(reason, centinelaPolicy) {
		t.Fatal("entregó una policy que no estaba cacheada")
	}
	if strings.Contains(reason, "POLICIES VIGENTES") {
		t.Fatalf("sin cache no puede anunciar una entrega vacía: %q", reason)
	}
}

// Un cache VACÍO no es lo mismo que un cache ausente, pero tampoco puede gastar la
// interrupción del usuario para no decir nada.
func TestPolicyDelivery_CacheVacio_NoGastaLaInterrupcion(t *testing.T) {
	home := t.TempDir()
	repo := repoGitDePrueba(t)
	escribirCachePolicies(t, home, repo, "")

	salida := correrHookEnRepo(t, "domain-pre-edit.sh", home, repo, payloadIntentoDeCommit(t))

	decision, _ := decisionDe(t, salida)
	if decision == "ask" {
		t.Fatal("un cache vacío no puede disparar la entrega: interrumpe sin contenido")
	}
}

// Regresión de formato: el CLI parsea UN solo objeto por invocación. Con dos, el helper de
// hook_destructive_guard_test.go y el propio cliente rompen.
func TestPolicyDelivery_SalidaEsUnSoloJSON(t *testing.T) {
	home := t.TempDir()
	repo := repoGitDePrueba(t)
	escribirCachePolicies(t, home, repo, centinelaPolicy)

	salida := correrHookEnRepo(t, "domain-pre-edit.sh", home, repo, payloadIntentoDeCommit(t))

	var v map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(salida)), &v); err != nil {
		t.Fatalf("la salida no es un único JSON: %v\nsalida: %q", err, salida)
	}
}
