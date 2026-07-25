// crowdsec_geo.go — DOMAINSERV-153.
//
// System cron: cada Tick consulta las alertas de la LAPI de CrowdSec y publica
// cuántos ataques vienen de cada país, para alimentar el panel Geomap del
// dashboard de seguridad.
//
// POR QUÉ LA LAPI Y NO LAS MÉTRICAS DE CROWDSEC: sus métricas Prometheus solo
// llevan los labels action/origin/reason — el país no está ahí. Y tampoco sale
// por stdout, así que Loki no lo tiene. El único lugar donde vive es el `source`
// de cada alerta.
//
// POR QUÉ CREDENCIAL DE MÁQUINA Y NO LA BOUNCER KEY: el endpoint de bouncer
// devuelve solo {duration,id,origin,scenario,scope,type,value}, sin geo. La doc
// de CrowdSec lo separa explícitamente: bouncers consultan decisiones, machines
// gestionan alertas. Verificado contra la LAPI real, no inferido.
package systemcron

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// countryUnknown agrupa las alertas cuyo país CrowdSec no pudo resolver (IPs
// privadas, rangos sin geolocalizar). Se etiquetan en vez de descartarse: si se
// descartaran, el total del panel no cerraría con los baneos reales.
const countryUnknown = "unknown"

// crowdsecHTTPTimeout acota cada llamada a la LAPI. Sin timeout un best-effort
// no es best-effort: es un cuelgue que se lleva puesto el tick.
const crowdsecHTTPTimeout = 10 * time.Second

type alertSource struct {
	CN     string `json:"cn"`
	IP     string `json:"ip"`
	ASName string `json:"as_name"`
}

type crowdsecAlert struct {
	Source alertSource `json:"source"`
}

// CrowdsecGeoCollector publica ataques por país leyendo la LAPI local.
//
// Degrada limpio por diseño: sin credencial, con la LAPI caída o si el login
// falla, loguea y sigue. El resto del server funciona igual sin este colector
// (policy llm-nunca-en-camino-caliente y su corolario sobre best-effort).
type CrowdsecGeoCollector struct {
	LAPIURL   string
	MachineID string
	Password  string
	Tick      time.Duration
	Logger    *slog.Logger

	// SetCountry recibe el conteo de cada país. Se inyecta desde el consumidor
	// para no acoplar el cron al registry de métricas.
	SetCountry func(cn string, count float64)

	client *http.Client
	token  string
}

// countByCountry agrega alertas por país. Pura y sin red a propósito: es la
// única lógica no trivial del colector y así se testea sin LAPI.
//
// Agrupa SOLO por país. La IP y el AS viajan en la alerta pero NO pueden entrar
// en la clave: como labels de métrica reventarían la cardinalidad y violarían
// low-cardinality-metrics.
func countByCountry(alerts []crowdsecAlert) map[string]int {
	out := make(map[string]int, len(alerts))
	for _, a := range alerts {
		cn := strings.TrimSpace(a.Source.CN)
		if cn == "" {
			cn = countryUnknown
		}
		out[cn]++
	}
	return out
}

func (c *CrowdsecGeoCollector) Start(ctx context.Context) {
	logger := c.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if c.MachineID == "" || c.Password == "" || c.LAPIURL == "" {
		logger.Info("crowdsec-geo deshabilitado: falta LAPI URL o credencial de máquina")
		return
	}
	if c.Tick == 0 {
		c.Tick = 5 * time.Minute
	}
	c.client = &http.Client{Timeout: crowdsecHTTPTimeout}
	logger.Info("crowdsec-geo started", slog.Duration("tick", c.Tick))

	c.runTick(ctx, logger)
	ticker := time.NewTicker(c.Tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("crowdsec-geo stopping")
			return
		case <-ticker.C:
			c.runTick(ctx, logger)
		}
	}
}

func (c *CrowdsecGeoCollector) runTick(ctx context.Context, logger *slog.Logger) {
	alerts, err := c.fetchAlerts(ctx)
	if err != nil {
		logger.Warn("crowdsec-geo tick falló, se reintenta el próximo", slog.Any("err", err))
		return
	}
	for cn, n := range countByCountry(alerts) {
		if c.SetCountry != nil {
			c.SetCountry(cn, float64(n))
		}
	}
}

// fetchAlerts hace login si no hay token y consulta las alertas. Un 401 vacía
// el token para que el próximo tick vuelva a loguearse: el JWT expira y
// reintentar con uno vencido fallaría siempre.
func (c *CrowdsecGeoCollector) fetchAlerts(ctx context.Context) ([]crowdsecAlert, error) {
	if c.token == "" {
		if err := c.login(ctx); err != nil {
			return nil, err
		}
	}
	alerts, status, err := c.getAlerts(ctx)
	if status == http.StatusUnauthorized {
		c.token = ""
		if err := c.login(ctx); err != nil {
			return nil, err
		}
		alerts, _, err = c.getAlerts(ctx)
	}
	return alerts, err
}

func (c *CrowdsecGeoCollector) login(ctx context.Context) error {
	body, err := json.Marshal(map[string]string{"machine_id": c.MachineID, "password": c.Password})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.LAPIURL+"/v1/watchers/login", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login LAPI devolvió %d", resp.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if out.Token == "" {
		return fmt.Errorf("login LAPI sin token en la respuesta")
	}
	c.token = out.Token
	return nil
}

func (c *CrowdsecGeoCollector) getAlerts(ctx context.Context) ([]crowdsecAlert, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.LAPIURL+"/v1/alerts", nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("GET /v1/alerts devolvió %d", resp.StatusCode)
	}
	var alerts []crowdsecAlert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return nil, resp.StatusCode, err
	}
	return alerts, resp.StatusCode, nil
}
