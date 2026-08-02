// Guard de coherencia de scheme entre MinIO y sus consumidores (DOMAINSERV-221).
//
// MinIO habilita TLS con solo detectar certs en /root/.minio/certs: no existe
// flag que lo pida ni log que avise del cambio. Entre el 18-jun-2026 y el
// 2026-08-02 el compose montaba esos certs mientras DOMAIN_S3_ENDPOINT seguia
// en http://, asi que toda operacion S3 del server moria con "connection reset
// by peer" y el bucket quedo vacio 45 dias sin que nadie lo notara.
//
// Este guard falla si alguien vuelve a mover una punta sin la otra.
package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	minioComposeRelPath = "services/minio/docker-compose.yml"
	mcpComposeRelPath   = "services/domain-mcp/docker-compose.yml"
	minioCertsMount     = "/root/.minio/certs"
)

// TestMinioCompose_SchemeDeLosConsumidores_CoincideConElModoDeServir deriva el
// scheme esperado de la presencia del mount de certs y lo exige a los tres
// consumidores: el cliente S3 del MCP, el healthcheck y el bootstrap del bucket.
func TestMinioCompose_SchemeDeLosConsumidores_CoincideConElModoDeServir(t *testing.T) {
	repoRoot := findRepoRootFromCwd(t)
	minioCompose := leerArchivoDelRepo(t, repoRoot, minioComposeRelPath)
	mcpCompose := leerArchivoDelRepo(t, repoRoot, mcpComposeRelPath)

	esperado := "http"
	if strings.Contains(minioCompose, minioCertsMount) {
		esperado = "https"
	}

	consumidores := []struct {
		nombre string
		marca  string
		fuente string
	}{
		{"DOMAIN_S3_ENDPOINT del cliente S3", "DOMAIN_S3_ENDPOINT:", mcpCompose},
		{"healthcheck de minio", "minio/health/live", minioCompose},
		{"mc alias set del bootstrap del bucket", "mc alias set", minioCompose},
	}

	for _, c := range consumidores {
		linea := lineaQueContiene(t, c.fuente, c.marca)
		require.Equal(t, esperado, schemeDeLaLinea(t, linea),
			"%s quedo desalineado: MinIO %s certs montados, asi que sirve %s://",
			c.nombre, presenciaDeCerts(esperado), esperado)
	}
}

func presenciaDeCerts(schemeEsperado string) string {
	if schemeEsperado == "https" {
		return "TIENE"
	}
	return "NO tiene"
}

func leerArchivoDelRepo(t *testing.T, repoRoot, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, relPath))
	require.NoError(t, err, "no se pudo leer %s", relPath)
	return string(data)
}

func lineaQueContiene(t *testing.T, contenido, marca string) string {
	t.Helper()
	for _, linea := range strings.Split(contenido, "\n") {
		if strings.Contains(linea, marca) {
			return linea
		}
	}
	t.Fatalf("ninguna linea contiene %q: el guard quedo apuntando a algo que ya no existe", marca)
	return ""
}

// schemeDeLaLinea chequea https primero: "https://" contiene "http".
func schemeDeLaLinea(t *testing.T, linea string) string {
	t.Helper()
	switch {
	case strings.Contains(linea, "https://"):
		return "https"
	case strings.Contains(linea, "http://"):
		return "http"
	}
	t.Fatalf("la linea no declara scheme http ni https: %q", linea)
	return ""
}
