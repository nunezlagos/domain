package main

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

type InstallOptions struct {
	Target              string // "" = auto-detect todos; "opencode"/"claude-code"/etc filtra
	AutoInstallOpencode bool   // si no hay clientes, instalar opencode
	NonInteractive      bool   // no prompt; errores fatales si falta decisión
}

type InstallPlan struct {
	Targets       []Client   // clientes a configurar (vacío si no se detectó ninguno)
	NeedsOpencode bool       // true si no hay clientes y hay que instalar
	OpencodeCmd   InstallCmd // comando sugerido para instalar opencode (si NeedsOpencode)
}

func BuildInstallPlan(opts InstallOptions) (InstallPlan, error) {
	p := DetectPlatform()

	detected := p.DetectedClients()
	if opts.Target != "" {
		var filtered []Client
		for _, c := range detected {
			if c.Name == opts.Target {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			return InstallPlan{}, fmt.Errorf("target '%s' no detectado. Disponibles: %s",
				opts.Target, clientNames(detected))
		}
		detected = filtered
	}

	if len(detected) == 0 {
		if !opts.AutoInstallOpencode {
			return InstallPlan{
				NeedsOpencode: true,
				OpencodeCmd:   InstallOpencodeCmd(p),
			}, nil
		}
		return InstallPlan{
			NeedsOpencode: true,
			OpencodeCmd:   InstallOpencodeCmd(p),
		}, nil
	}

	return InstallPlan{
		Targets:       detected,
		NeedsOpencode: false,
	}, nil
}

func clientNames(cs []Client) string {
	names := make([]string, len(cs))
	for i, c := range cs {
		names[i] = c.Name
	}
	return fmt.Sprintf("%v", names)
}

// pingVPS hace GET /healthz y reporta error si status >= 500 o timeout.
func pingVPS(ctx context.Context, baseURL string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("ping failed: status %d", resp.StatusCode)
	}
	return nil
}

// ApplyResult reporta, por cliente, si se configuró o se omitió (y por qué).
type ApplyResult struct {
	Client  string
	Skipped bool
	Reason  string
}

// Apply ejecuta la configuración de los Targets con la URL+key.
// No instala opencode ni hace ping (eso es responsabilidad del caller).
// Devuelve un resultado por cliente para que el caller informe skips/dedup.
func Apply(plan InstallPlan, vpsURL, apiKey string) ([]ApplyResult, error) {
	if len(plan.Targets) == 0 {
		return nil, fmt.Errorf("Apply: plan sin targets")
	}
	ts := Timestamp()
	results := make([]ApplyResult, 0, len(plan.Targets))
	for _, c := range plan.Targets {
		res, err := configureClient(c, vpsURL, apiKey, ts)
		if err != nil {
			return results, fmt.Errorf("%s: %w", c.Name, err)
		}
		results = append(results, ApplyResult{Client: c.Name, Skipped: res.Skipped, Reason: res.Reason})
	}
	return results, nil
}

// runInstallCmd ejecuta InstallCmd.Primary; si falla corre Fallback.
// Solo se usa si el operador pasa --install-opencode (decisión humana).
func runInstallCmd(cmd InstallCmd) error {
	if len(cmd.Primary) > 0 {
		if err := exec.Command(cmd.Primary[0], cmd.Primary[1:]...).Run(); err == nil {
			return nil
		}
	}
	if len(cmd.Fallback) == 0 {
		return fmt.Errorf("no hay fallback para ejecutar")
	}
	return exec.Command(cmd.Fallback[0], cmd.Fallback[1:]...).Run()
}
