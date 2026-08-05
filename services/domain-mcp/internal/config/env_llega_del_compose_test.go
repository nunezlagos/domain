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

// DOMAIN_MASTER_KEY cifra el env de mcp_servers y firma los secretos de los webhooks
// outbound. server_services.go la lee de os.Getenv y, si falta, ni construye el cipher:
// los webhooks responden 503 permanente (DOMAINSERV-231). Medido 2026-08-04 con docker
// inspect: no estaba en el env del container ni en el .env del VPS.
func TestCompose_MasterKey_LlegaAlContainer(t *testing.T) {
	compose := leerCompose(t)

	if !regexp.MustCompile(`(?m)^\s*DOMAIN_MASTER_KEY:`).MatchString(compose) {
		t.Error("el compose no declara DOMAIN_MASTER_KEY: el cipher no se construye y los webhooks outbound fallan")
	}
}

// crypto.LoadFromBase64 decodifica con base64.StdEncoding y NewCipher exige
// MasterKeySize=32 bytes, así que el gen_uuid del array CREDS no sirve para esta clave:
// un UUID no es base64 de 32 bytes. Necesita su propio generador.
func TestInstallSh_MasterKey_SeGeneraConBase64DeTreintaYDosBytes(t *testing.T) {
	install := leerArchivoDelRepo(t, "install.sh")

	generador := regexp.MustCompile(`gen_master_key\(\)`)
	if !generador.MatchString(install) {
		t.Fatal("install.sh no define gen_master_key: sin generador propio la clave saldría como UUID y LoadFromBase64 la rechaza")
	}

	cuerpo := regexp.MustCompile(`gen_master_key\(\)\s*\{[^}]*\}`).FindString(install)
	if !strings.Contains(cuerpo, "head -c 32 /dev/urandom") {
		t.Errorf("gen_master_key debe leer 32 bytes de /dev/urandom (MasterKeySize=32), cuerpo: %q", cuerpo)
	}
	if !strings.Contains(cuerpo, "base64") {
		t.Errorf("gen_master_key debe codificar en base64 (LoadFromBase64 usa StdEncoding), cuerpo: %q", cuerpo)
	}

	if !regexp.MustCompile(`DOMAIN_MASTER_KEY.*gen_master_key|gen_master_key.*DOMAIN_MASTER_KEY`).MatchString(install) {
		t.Error("install.sh define gen_master_key pero no la usa para DOMAIN_MASTER_KEY")
	}
}

// DOMAINSERV-243: la paridad entre los dos .env.example es lo que arregla la CLASE,
// no el caso. Son dos caminos distintos y los dos son reales: services/.env.example
// lo consume install.sh en el deploy, y services/domain-mcp/.env.example es el de
// `go run ./cmd/domain server` en local. DOMAIN_MASTER_KEY se documentó en el
// primero y se olvidó en el segundo, así que un dev que levanta local reproduce el
// 503 de DOMAINSERV-231 sin una línea que se lo explique.
//
// El invariante NO es "los 13 secretos de CREDS en los dos archivos": varios son de
// infra (Grafana, CrowdSec, el backup, MinIO) o de otro servicio (Django, el panel)
// y no tienen nada que hacer en el example del server. El invariante correcto se
// deriva del compose: si el service domain-mcp DECLARA un secreto, entonces el
// server lo consume, y su example dev-local tiene que documentarlo.
func TestEnvExample_ParidadDeSecretosQueElServerConsume(t *testing.T) {
	credsDeInstalador := clavesDeCREDS(t)
	if len(credsDeInstalador) == 0 {
		t.Fatal("no se pudo parsear el array CREDS de install.sh: sin eso este guard no mide nada")
	}
	bloqueDelServer := leerCompose(t)
	exampleDev := leerArchivoDelRepo(t, filepath.Join("domain-mcp", ".env.example"))

	var faltan []string
	for _, clave := range credsDeInstalador {
		enElCompose := regexp.MustCompile(`(?m)^\s*` + clave + `:`).MatchString(bloqueDelServer)
		if !enElCompose {
			continue
		}
		declarada := regexp.MustCompile(`(?m)^` + clave + `=`).MatchString(exampleDev)
		if !declarada {
			faltan = append(faltan, clave)
		}
	}
	if len(faltan) > 0 {
		t.Errorf("el service domain-mcp declara estos secretos en su compose pero su .env.example dev-local no los documenta: %s\n"+
			"un dev que levanta local con ese archivo arranca sin ellos y reproduce el fallo en silencio",
			strings.Join(faltan, ", "))
	}
}

// El acotamiento de leerCompose necesita su propio test: el de paridad NO lo mide.
// Se comprobó por sabotaje — al devolver hasta EOF de nuevo, el de paridad siguió en
// verde, porque OPENCODE_SERVER_PASSWORD está declarada en los DOS services y da
// igual dónde se la busque. Sin este caso, el acotamiento sería un cambio sin guard:
// alguien lo revierte, los asserts de este archivo vuelven a mirar el compose entero
// y una variable declarada bajo `opencode` pasa como si estuviera en el server.
func TestLeerCompose_DevuelveSoloElBloqueDelServiceDomainMcp(t *testing.T) {
	bloque := leerCompose(t)

	if strings.Contains(bloque, "  opencode:") {
		t.Error("leerCompose incluye el service opencode: un assert sobre este texto no puede afirmar nada del server")
	}
	if regexp.MustCompile(`(?m)^networks:`).MatchString(bloque) {
		t.Error("leerCompose llega hasta el final del archivo en vez de cortar en el próximo service")
	}
	// contra-prueba: si acotara DE MÁS, el bloque quedaría vacío y todos los asserts
	// de este archivo pasarían por vacuidad
	if !regexp.MustCompile(`(?m)^\s*DOMAIN_MASTER_KEY:`).MatchString(bloque) {
		t.Error("el bloque quedó sin el env del propio service: acotó de más y los asserts pasarían por vacuidad")
	}
}

// clavesDeCREDS extrae los nombres del array asociativo de install.sh. Se parsea el
// instalador en vez de mantener una lista acá: una lista paralela se desincroniza y
// el guard pasaría a proteger un conjunto que ya no es el real.
func clavesDeCREDS(t *testing.T) []string {
	t.Helper()
	install := leerArchivoDelRepo(t, "install.sh")
	inicio := strings.Index(install, "declare -A CREDS=(")
	if inicio == -1 {
		t.Fatal("install.sh no declara el array CREDS")
	}
	fin := strings.Index(install[inicio:], "\n)")
	if fin == -1 {
		t.Fatal("no se encontró el cierre del array CREDS")
	}
	var claves []string
	for _, m := range regexp.MustCompile(`\[([A-Z_]+)\]=`).FindAllStringSubmatch(install[inicio:inicio+fin], -1) {
		claves = append(claves, m[1])
	}
	return claves
}

// leerCompose devuelve SOLO el bloque del service domain-mcp.
//
// Antes devolvía desde "  domain-mcp:" hasta EOF, y eso hacía que una variable
// declarada bajo `opencode:` (donde el cipher del server no existe) pasara los
// asserts de este archivo como si estuviera en el service correcto. Acotarlo al
// siguiente key de nivel 2 es lo que hace que los guards midan el service que dicen
// medir (DOMAINSERV-243, misma familia que el 231 que los escribió).
func leerCompose(t *testing.T) string {
	t.Helper()
	crudo := leerArchivoDelRepo(t, filepath.Join("domain-mcp", "docker-compose.yml"))
	inicio := strings.Index(crudo, "  domain-mcp:")
	if inicio == -1 {
		t.Fatal("el compose no tiene un service domain-mcp")
	}
	bloque := crudo[inicio+len("  domain-mcp:"):]
	if fin := regexp.MustCompile(`(?m)^  [a-zA-Z_-]+:`).FindStringIndex(bloque); fin != nil {
		bloque = bloque[:fin[0]]
	}
	return bloque
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
