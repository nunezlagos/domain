package main

import (
	"strings"
	"testing"
)

// esDelGitGuard distingue el GIT-guard del guard destructivo de rm: esDelGuard() del otro archivo
// busca "destructive-guard" y no matchea este, así que usarlo acá daba un falso "dejó pasar"
// cuando el hook había denegado correctamente.
func esDelGitGuard(d decisionHook) bool {
	return strings.Contains(d.Razon, "git-guard")
}

// DOMAINSERV-247, incidencia encontrada al medir el plugin de OpenCode: el git-guard de BASH
// tiene el mismo defecto de exceso. Se descubrió en vivo — este guard rechazó un comando de
// diagnóstico que solo MENCIONABA "git clean -n" dentro de un script de node.
//
// DOMAINSERV-111 ya había excluido `stash list` y `stash show`, pero dejó `git clean` con el
// patrón amplio, así que `-n` y `--dry-run` seguían bloqueados. El arreglo quedó a medias y esto
// lo termina.
//
// Estos tests EJECUTAN el hook real con decisionDeBash, igual que el resto del guard: leer el
// patrón no prueba cómo clasifica.

func TestGitGuardBash_LecturasDeCleanYStash_NoDisparan(t *testing.T) {
	// se arman por concatenación para que el propio guard no bloquee este archivo al leerlo:
	// clasifica por substring, y bloquearía el test que denuncia exactamente eso
	g := "git "
	lecturas := []struct{ nombre, cmd string }{
		{"clean_dry_run_corto", g + "cl" + "ean -n"},
		{"clean_dry_run_largo", g + "cl" + "ean --dry-run"},
		{"stash_list", g + "st" + "ash list"},
		{"stash_show", g + "st" + "ash show"},
	}

	for _, c := range lecturas {
		t.Run(c.nombre, func(t *testing.T) {
			d := decisionDeBash(t, dirLimpio(t), c.cmd, "bypassPermissions")
			if esDelGitGuard(d) {
				t.Errorf("el guard bloqueó una LECTURA: %s\nrazon=%q\n"+
					"Un guard que impide mirar el estado obliga a trabajar a ciegas y empuja a "+
					"desactivarlo entero (DOMAINSERV-111/175/195)", c.cmd, d.Razon)
			}
		})
	}
}

// Afinar el patrón no puede abrir la puerta que el guard vino a cerrar.
//
// DOMAINSERV-278: estos casos se afirman contra d.Decision, NO contra esDelGitGuard.
// Medirlo por la razón era un falso verde: al reclasificar `stash pop` de deny a ask, la
// razón sigue conteniendo "git-guard", así que el test pasaba en verde con el
// comportamiento cambiado. La razón dice QUIÉN decidió; solo la decisión dice QUÉ se
// decidió.
func TestGitGuardBash_MutacionesDeCleanYStash_SiguenDisparando(t *testing.T) {
	g := "git "
	mutaciones := []struct{ nombre, cmd, esperada string }{
		{"clean_forzado", g + "cl" + "ean -fd", "deny"},
		{"clean_f", g + "cl" + "ean -f", "deny"},
		{"stash_drop", g + "st" + "ash drop", "deny"},
		{"stash_clear", g + "st" + "ash clear", "deny"},
		{"worktree_remove_force", g + "worktree remove --force x", "deny"},
		{"worktree_remove_force_segunda_posicion", g + "worktree remove x --force", "deny"},
		// recuperables: el humano decide en el diálogo, pero el guard TIENE que opinar
		{"stash_pop", g + "st" + "ash pop", "ask"},
		{"stash_pelado", g + "st" + "ash", "ask"},
		{"restore", g + "re" + "store .", "ask"},
		{"worktree_remove_limpio", g + "worktree remove x", "ask"},
	}

	for _, c := range mutaciones {
		t.Run(c.nombre, func(t *testing.T) {
			d := decisionDeBash(t, dirLimpio(t), c.cmd, "bypassPermissions")
			if !esDelGitGuard(d) {
				t.Fatalf("DEJÓ PASAR una mutación: %s\ndecision=%q razon=%q",
					c.cmd, d.Decision, d.Razon)
			}
			if d.Decision != c.esperada {
				t.Errorf("%s: decidió %q y se esperaba %q.\nrazon=%q\n"+
					"deny e irrecuperable van juntos: si algo recuperable cae en deny el usuario "+
					"se queda sin camino, y si algo irrecuperable cae en ask se puede aprobar de "+
					"un click lo que no se revierte", c.cmd, d.Decision, c.esperada, d.Razon)
			}
		})
	}
}
