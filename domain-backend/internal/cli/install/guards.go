package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// projectRootMarkers son los archivos que DEBEN existir en un
// directorio para considerarlo root del repo domain. Si falta
// alguno, IsProjectRoot retorna la lista de los faltantes.
var projectRootMarkers = []string{
	".env.example",
	"docker-compose.yml",
}

// IsProjectRoot retorna (true, nil, nil) si el directorio en path
// contiene TODOS los archivos en projectRootMarkers. Si falta
// alguno, retorna (false, [faltantes], nil). Si el path no es
// accesible o no es un directorio, retorna (false, nil, err).
//
// El caller usa la lista de missing para armar el mensaje de
// error accionable (no es un string hardcodeado).
func IsProjectRoot(path string) (bool, []string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.IsDir() {
		return false, nil, fmt.Errorf("path %s is not a directory", path)
	}

	var missing []string
	for _, marker := range projectRootMarkers {
		_, err := os.Stat(filepath.Join(path, marker))
		if err == nil {
			continue
		}
		if os.IsNotExist(err) {
			missing = append(missing, marker)
			continue
		}
		return false, nil, fmt.Errorf("stat %s/%s: %w", path, marker, err)
	}

	if len(missing) > 0 {
		return false, missing, nil
	}
	return true, nil, nil
}
