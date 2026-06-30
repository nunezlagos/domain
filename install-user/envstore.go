package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type EnvData struct {
	VPSURL string
	Email  string
}

func loadEnv(path string) (EnvData, error) {
	var data EnvData
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return data, nil
	}
	if err != nil {
		return data, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"'`)
		switch strings.TrimSpace(k) {
		case "DOMAIN_VPS_URL":
			data.VPSURL = v
		case "DOMAIN_USER_EMAIL":
			data.Email = v
		}
	}
	return data, sc.Err()
}

func saveEnv(path string, data EnvData) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(
		"# domain install — generado por domain-install\n"+
			"# API_KEY no se guarda acá por seguridad.\n"+
			"DOMAIN_VPS_URL=%q\nDOMAIN_USER_EMAIL=%q\n",
		data.VPSURL, data.Email,
	)
	return os.WriteFile(path, []byte(content), 0o600)
}

func removeEnv(path string) {
	_ = os.Remove(path)
}

// detectURLMismatch lee install.env y compara la URL guardada con newURL.
// Retorna warning string si difieren (no es error — el operador decide).
// Retorna "" si son iguales, archivo no existe, o no se puede parsear.
func detectURLMismatch(path, newURL string) (string, error) {
	data, err := loadEnv(path)
	if err != nil {
		return "", err
	}
	if data.VPSURL == "" {
		return "", nil
	}
	if data.VPSURL == newURL {
		return "", nil
	}
	return "install.env tiene VPS_URL='" + data.VPSURL +
		"' pero pediste '" + newURL + "'. ¿Re-apuntar? (Y/n)", nil
}
