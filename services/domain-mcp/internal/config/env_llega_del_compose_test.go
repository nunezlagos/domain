package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// DOMAIN_ENV decide si checkOrphanPolicy (runner.go) rechaza runs sin flow_run_id.
// El default de config.go es "dev", así que mientras la var no esté en el compose
// prod corre con la barrera apagada — y eso ya pasó (DOMAINSERV-236, medido
// 2026-08-04 con docker inspect: la var no estaba en el env del container).
// El default va en el compose y NO en el .env porque install.sh preserva el .env
// viejo sin agregarle claves nuevas: una var solo declarada en el example nunca
// llega a una instalación existente.
func TestCompose_DomainEnv_TieneDefaultProd(t *testing.T) {
	compose := leerCompose(t)

	linea := regexp.MustCompile(`(?m)^\s*DOMAIN_ENV:\s*(.+)$`).FindStringSubmatch(compose)
	if linea == nil {
		t.Fatal("el compose no declara DOMAIN_ENV: prod corre con el default dev de config.go y checkOrphanPolicy queda desactivada")
	}

	valor := strings.TrimSpace(linea[1])
	if !strings.Contains(valor, "DOMAIN_ENV:-prod") {
		t.Errorf("DOMAIN_ENV debe defaultear a prod para que un deploy limpio nazca seguro, no %q", valor)
	}
}

// El password del sidecar opencode viaja por la red interna de Docker. Con el
// default vacío del compose, el basic auth queda sin secreto y solo install.sh
// puede poblarlo: es un secreto, así que va en el array CREDS del instalador.
func TestInstallSh_OpencodePassword_LaGeneraElInstalador(t *testing.T) {
	install := leerArchivoDelRepo(t, "install.sh")

	if !regexp.MustCompile(`\[OPENCODE_SERVER_PASSWORD\]=OPENCODE_SERVER_PASSWORD`).MatchString(install) {
		t.Error("OPENCODE_SERVER_PASSWORD no está en el array CREDS de install.sh: el basic auth de opencode se despliega sin secreto")
	}
}

func leerCompose(t *testing.T) string {
	t.Helper()
	crudo := leerArchivoDelRepo(t, filepath.Join("domain-mcp", "docker-compose.yml"))
	inicio := strings.Index(crudo, "  domain-mcp:")
	if inicio == -1 {
		t.Fatal("el compose no tiene un service domain-mcp")
	}
	return crudo[inicio:]
}

func leerArchivoDelRepo(t *testing.T, relativo string) string {
	t.Helper()
	ruta := filepath.Join("..", "..", "..", relativo)
	contenido, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", relativo, err)
	}
	return string(contenido)
}
