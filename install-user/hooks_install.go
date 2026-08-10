package main

import (
	"bytes"
	"os"
	"path/filepath"
)

// resultadoHook dice qué pasó con un hook al intentar instalarlo. Se devuelve en vez de solo un
// error porque "no hice nada porque ya estaba bien" y "no hice nada porque el disco difiere" son
// desenlaces distintos: el segundo hay que reportarlo.
type resultadoHook int

const (
	hookAlDia      resultadoHook = iota // el disco ya tiene exactamente lo embebido
	hookEscrito                         // no existía y se instaló
	hookDivergente                      // existe y difiere: NO se pisa
	hookFallo                           // no se pudo leer el embebido o escribir el destino
)

func (r resultadoHook) String() string {
	switch r {
	case hookAlDia:
		return "al-dia"
	case hookEscrito:
		return "escrito"
	case hookDivergente:
		return "divergente"
	default:
		return "fallo"
	}
}

// instalarHookEmbebido escribe un lifecycle hook desde el binario, decidiendo por PROCEDENCIA
// de qué es la diferencia cuando el disco no coincide (DOMAINSERV-267).
//
// CAMBIO DE POLÍTICA respecto de DOMAINSERV-239, deliberado. El 239 no pisaba NINGÚN archivo
// divergente porque no tenía con qué distinguir "quedó viejo" de "lo editó el usuario": ante
// la duda, no tocar. Era correcto para lo que ese ticket atacaba —que un binario de un tag
// anterior degradara el gate SDD de una máquina al día— pero convirtió el canal de
// distribución en uno que no distribuye: un fix de hook no llegaba a ninguna máquina, y el
// install lo reportaba como éxito.
//
// El manifiesto resuelve la duda que el 239 no podía resolver, y es el MISMO mecanismo que
// los agentes ya usan (agent_manifest.go). clasificar() devuelve:
//   - alDia:      el disco ya es el embebido, no se toca
//   - deDomain:   lo escribió domain (o no hay registro previo) → se pisa
//   - delUsuario: el disco no es el embebido NI lo que domain registró → lo editó alguien,
//     se respeta y se avisa
//
// El caso "sin registro previo → se pisa" es la política de gracia decidida con el usuario:
// hoy ninguna máquina tiene manifiesto, así que respetar por defecto habría dejado a todo el
// parque sin poder actualizarse salvo borrando hooks a mano. Se pisa una vez, con backup, y
// desde la segunda corrida el manifiesto ya distingue.
func instalarHookEmbebido(script, hooksDir string, manifiesto manifiestoAgentes) resultadoHook {
	embebido, err := hooksFS.ReadFile("hooks/" + script)
	if err != nil {
		return hookFallo
	}
	destino := filepath.Join(hooksDir, script)

	if _, rerr := os.Stat(destino); rerr == nil {
		switch clasificar(destino, embebido, manifiesto[script]) {
		case alDia:
			// se registra igual: un hook idéntico al embebido y sin entrada en el manifiesto
			// quedaría clasificado como "del usuario" en cuanto el binario cambie
			manifiesto[script] = sha256Hex(embebido)
			return hookAlDia
		case delUsuario:
			return hookDivergente
		}
		// deDomain: se pisa, pero el contenido anterior no se pierde
		if err := respaldarHook(destino); err != nil {
			return hookFallo
		}
	}

	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return hookFallo
	}
	// 0o755 y no 0o644: un hook sin permiso de ejecución no corre, y ese fallo es mudo
	if err := escribirHookAtomico(destino, embebido); err != nil {
		return hookFallo
	}
	manifiesto[script] = sha256Hex(embebido)
	return hookEscrito
}

// respaldarHook copia el hook actual junto al original antes de pisarlo. El backup es lo que
// hace reversible la política de gracia: la primera corrida pisa sin poder distinguir una
// edición del usuario, y sin copia esa edición se perdería sin rastro.
func respaldarHook(destino string) error {
	actual, err := os.ReadFile(destino)
	if err != nil {
		return err
	}
	return os.WriteFile(destino+".bak-"+Timestamp(), actual, 0o755)
}

// escribirHookAtomico escribe por temp+rename en el MISMO directorio. Un os.WriteFile directo
// deja el hook truncado si el proceso muere a mitad, y un hook truncado no falla: corre y
// decide mal, que es peor que no estar.
func escribirHookAtomico(destino string, contenido []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(destino), ".hook-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(contenido); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), destino)
}

// divergeDelEmbebido dice si el hook en disco NO es el que trae el binario.
//
// Ante la duda devuelve false: si no se puede leer el embebido o el archivo, no hay evidencia de
// divergencia y afirmarla sería ruido. La ausencia del archivo ya la reporta fileExists.
func divergeDelEmbebido(script, hookPath string) bool {
	embebido, err := hooksFS.ReadFile("hooks/" + script)
	if err != nil {
		return false
	}
	actual, err := os.ReadFile(hookPath)
	if err != nil {
		return false
	}
	return !bytes.Equal(actual, embebido)
}

// instalarHooksLifecycle instala los hooks que claudeHooks declara y reporta el desenlace de cada
// uno. Devuelve la cantidad de divergentes, para que el caller decida si eso merece ruido.
//
// DOMAINSERV-239, modo de falla 1: esto corre en la zona de installGlobalAssets y NO al final del
// install. runInstall tiene os.Exit(1) más abajo, y si los hooks se instalaran después de esos
// puntos, un fallo en cualquiera de ellos dejaría la máquina SIN hooks en un install limpio —
// donde hoy quedaban correctos porque los escribía el script antes de invocar al binario.
func instalarHooksLifecycle(hooksDir, manifestPath string) int {
	// domain-hooks-lib.sh NO está en claudeHooks porque no es un hook registrado: es la lib que
	// TODOS cargan con `. "$LIB"`. Iterar solo claudeHooks la dejaría afuera y los hooks fallarían
	// en su primera línea útil, con un error mudo que se ve como "el gate no anda".
	archivos := []string{"domain-hooks-lib.sh"}
	for _, spec := range claudeHooks {
		archivos = append(archivos, spec.Script)
	}

	manifiesto := cargarManifiesto(manifestPath)

	divergentes := 0
	for _, script := range archivos {
		switch instalarHookEmbebido(script, hooksDir, manifiesto) {
		case hookEscrito:
			ok("hook instalado: " + script)
		case hookDivergente:
			divergentes++
			// el mensaje NO dice "borralo y reinstalá": con un binario viejo, obedecer esa
			// instrucción degrada el gate SDD a la versión del binario — el modo de falla
			// ejecutado por el propio remedio (DOMAINSERV-267)
			warnL("hook con cambios locales, se respeta y NO se actualiza: " + script +
				" — si no lo editaste vos, guardá una copia y borralo para que el instalador lo reponga")
		case hookFallo:
			warnL("no se pudo instalar el hook: " + script)
		}
	}

	// se persiste una vez al final y no por hook: si algo falla en el medio, el manifiesto no
	// queda declarando hooks que no se escribieron
	if err := guardarManifiesto(manifestPath, manifiesto); err != nil {
		warnL("no se pudo guardar el manifiesto de hooks: " + err.Error() +
			" — la próxima corrida no va a poder distinguir un hook viejo de uno editado")
	}
	return divergentes
}
