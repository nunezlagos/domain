// Package failover — cadena de providers LLM por rol con degradación en runtime.
//
// Un Provider (primario) que falla por un error retryable (429/5xx/timeout) o
// por circuito abierto rota al siguiente de la cadena antes de degradar. Un
// error fatal (400/auth) corta la cadena de inmediato (no tiene sentido
// reintentar la misma request en otro provider si el input es inválido).
package failover

import (
	"context"
	"errors"
	"os"
	"strings"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/circuitbreaker"
	"nunezlagos/domain/internal/llm/retry"
)

// Observer recibe (provider, result) por cada intento: result ∈
// {success, failover, fatal}. Baja cardinalidad (enums). Puede ser nil.
type Observer func(provider, result string)

// Provider implementa llm.Provider iterando una cadena de providers.
type Provider struct {
	name  string
	chain []llm.Provider
	obs   Observer
}

// New crea un Provider de failover sobre una cadena ordenada [primary, ...].
func New(name string, chain []llm.Provider, obs Observer) *Provider {
	return &Provider{name: name, chain: chain, obs: obs}
}

func (p *Provider) Name() string { return p.name }

// shouldFailover indica si conviene rotar al siguiente provider: errores
// transitorios (retry.IsTransient) o circuito abierto. Un error fatal (auth /
// 4xx) NO rota — se devuelve tal cual.
func shouldFailover(err error) bool {
	return retry.IsTransient(err) || errors.Is(err, circuitbreaker.ErrCircuitOpen)
}

func (p *Provider) notify(provider, result string) {
	if p.obs != nil {
		p.obs(provider, result)
	}
}

func (p *Provider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	var lastErr error
	for _, prov := range p.chain {
		resp, err := prov.Complete(ctx, opts)
		if err == nil {
			p.notify(prov.Name(), "success")
			return resp, nil
		}
		lastErr = err
		if !shouldFailover(err) {
			p.notify(prov.Name(), "fatal")
			return nil, err
		}
		p.notify(prov.Name(), "failover")
	}
	return nil, lastErr
}

func (p *Provider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	var lastErr error
	for _, prov := range p.chain {
		ch, err := prov.CompleteStream(ctx, opts)
		if err == nil {
			p.notify(prov.Name(), "success")
			return ch, nil
		}
		lastErr = err
		if !shouldFailover(err) {
			p.notify(prov.Name(), "fatal")
			return nil, err
		}
		p.notify(prov.Name(), "failover")
	}
	return nil, lastErr
}

// ForRole resuelve el provider de un rol con failover. Si
// DOMAIN_LLM_<ROL>_CHAIN está seteado (lista de nombres separados por coma),
// construye una cadena y devuelve model="" (cada provider usa su modelo por
// default). Si no, delega en Factory.ProviderForRole (provider único + modelo
// del rol) — sin overhead de failover.
func ForRole(f *llm.Factory, role llm.Role, obs Observer) (llm.Provider, string, error) {
	raw := strings.TrimSpace(os.Getenv(chainEnv(role)))
	if raw == "" {
		return f.ProviderForRole(role)
	}
	var chain []llm.Provider
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if p, err := f.Get(name); err == nil {
			chain = append(chain, p)
		}
	}
	switch len(chain) {
	case 0:
		return f.ProviderForRole(role)
	case 1:
		return chain[0], "", nil
	default:
		return New("failover:"+string(role), chain, obs), "", nil
	}
}

func chainEnv(role llm.Role) string {
	return "DOMAIN_LLM_" + strings.ToUpper(string(role)) + "_CHAIN"
}
