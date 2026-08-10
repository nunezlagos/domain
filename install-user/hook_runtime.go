package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Primitivas de los lifecycle hooks, portadas de hooks/domain-hooks-lib.sh (DOMAINSERV-273).
//
// El port existe porque los hooks en .sh no corren en Windows —el registro pone el path del
// script como `command`, sin bash— y porque dependen de 8 utilidades externas que no están en
// todas las plataformas. En Go el binario ya es cross-platform y no invoca nada de afuera.
//
// Solo está acá lo que el hook Stop necesita. Los otros hooks van a pedir más primitivas; se
// agregan cuando se porten, no antes.

// credencialesDomain son la URL del server y la API key con las que un hook habla con el MCP.
type credencialesDomain struct {
	VPSURL string
	APIKey string
}

// resolverCredenciales replica el orden de domain_resolve_env: primero el entorno, después los
// tres .env que escribe el instalador. El primero que aparece gana, y no se pisa con los
// siguientes — un valor exportado a mano tiene que poder ganarle al archivo.
func resolverCredenciales(home string) (credencialesDomain, bool) {
	c := credencialesDomain{
		VPSURL: os.Getenv("DOMAIN_VPS_URL"),
		APIKey: os.Getenv("DOMAIN_API_KEY"),
	}
	for _, ruta := range []string{
		filepath.Join(home, ".config", "domain", "install.env"),
		filepath.Join(home, ".claude", ".env"),
		filepath.Join(home, ".config", "opencode", ".env"),
	} {
		if c.VPSURL != "" && c.APIKey != "" {
			break
		}
		leerEnvA(ruta, &c)
	}
	return c, c.VPSURL != "" && c.APIKey != ""
}

func leerEnvA(ruta string, c *credencialesDomain) {
	f, err := os.Open(ruta)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		clave, valor, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		valor = strings.Trim(strings.TrimSpace(valor), `"'`)
		switch strings.TrimSpace(clave) {
		case "DOMAIN_VPS_URL":
			if c.VPSURL == "" {
				c.VPSURL = valor
			}
		// DOMAIN_MCP_API_KEY es el nombre con el que el instalador escribe la key en los .env
		// de los clientes; DOMAIN_API_KEY es el del entorno. Los dos son la misma key.
		case "DOMAIN_MCP_API_KEY", "DOMAIN_API_KEY":
			if c.APIKey == "" {
				c.APIKey = valor
			}
		}
	}
}

// llamarTool invoca un tool del MCP por JSON-RPC y devuelve el cuerpo de la respuesta.
//
// El Bearer va como header y no por línea de comandos: el equivalente en bash necesitaba
// `-K <(printf ...)` para que la key no quedara visible en argv (DOMAINSERV-77). En Go ese
// workaround deja de hacer falta.
func llamarTool(c credencialesDomain, tool string, args any, timeout time.Duration) (string, error) {
	cuerpo, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": args},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(c.VPSURL, "/")+"/mcp", bytes.NewReader(cuerpo))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out bytes.Buffer
	if _, err := out.ReadFrom(resp.Body); err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return out.String(), fmt.Errorf("http %d", resp.StatusCode)
	}
	return out.String(), nil
}

// registrarErrorDeHook deja el fallo en hook-errors.log. Un hook es best-effort y nunca
// bloquea, así que sin este rastro un fallo del server sería completamente invisible
// (REQ-56 issue-56.2).
func registrarErrorDeHook(home, hook, sessionID, operacion, detalle string) {
	dir := filepath.Join(home, ".local", "state", "domain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "hook-errors.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\t%s\t%s\t%s\t%s\n", Timestamp(), hook, sessionID, operacion, unaLinea(detalle))
}

// unaLinea aplasta saltos y tabs: cada error ocupa UNA línea del log, o el formato TSV deja
// de ser parseable en cuanto el server devuelve un JSON multilínea.
func unaLinea(s string) string {
	return strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(s)
}
