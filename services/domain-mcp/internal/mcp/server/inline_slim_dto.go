package mcpserver

import (
	agentsvc "nunezlagos/domain/internal/service/agent"
	capturedpromptsvc "nunezlagos/domain/internal/service/capturedprompt"
	flowsvc "nunezlagos/domain/internal/service/flow"
	projectpolicysvc "nunezlagos/domain/internal/service/projectpolicy"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

// Mismo idiom que ticketSlim (ver ticket_slim_dto.go), aplicado a los structs que NO
// declaran tags json: sus keys salen con el nombre de campo Go en PascalCase, así que
// el tag del campo shadowed tiene que replicar ese nombre exacto o el cuerpo sigue
// saliendo. Se preserva el naming a propósito (DOMAINSERV-161, ADR-161.1): normalizar a
// snake_case cambiaría TODAS las keys y no solo el cuerpo.
// El shadowing depende del nombre del campo en el struct de origen: si se renombra,
// el cuerpo vuelve al JSON sin error de compilación.

type skillSlim struct {
	skillsvc.Skill
	Content    campoOmitido `json:"Content,omitempty"`
	ContentLen int
}

// SearchResult embebe Skill, así que Content queda a profundidad 2 y el shadowing de
// profundidad 0 lo tapa igual. Se omite en vez de truncar porque una skill se elige por
// slug y descripción, no por su cuerpo; el snippet acotado solo aplica a las tools cuyo
// consumidor lee el texto (mem_search, mem_context, knowledge_search).
type skillSearchSlim struct {
	skillsvc.SearchResult
	Content    campoOmitido `json:"Content,omitempty"`
	ContentLen int
}

type agentSlim struct {
	agentsvc.Agent
	SystemPrompt    campoOmitido `json:"SystemPrompt,omitempty"`
	SystemPromptLen int
}

// Spec no es texto sino un struct con los Steps y sus Config: el largo en caracteres no
// dice nada útil, y la versión más el conteo de pasos sí alcanzan para decidir si vale
// pedir el detalle con domain_flow_list de un slug puntual.
type flowSlim struct {
	flowsvc.Flow
	Spec           campoOmitido `json:"Spec,omitempty"`
	SpecVersion    int
	SpecStepsCount int
}

// A diferencia de los tres de arriba, estos dos structs SÍ declaran tags json en
// snake_case, así que el shadowing replica ese nombre y no el del campo Go.

// body_structured también se omite: es el mismo cuerpo en otra forma. Se conservan slug,
// name, kind, version y override_platform porque el prompt de sdd-review depende del
// último para decidir si la policy de proyecto pisa a la de plataforma.
type policySlim struct {
	projectpolicysvc.Policy
	BodyMD         campoOmitido `json:"body_md,omitempty"`
	BodyStructured campoOmitido `json:"body_structured,omitempty"`
	BodyMDLen      int          `json:"body_md_len"`
}

// CharCount ya existe en el struct de origen: no se agrega un campo de longitud nuevo
type promptCapturadoSlim struct {
	capturedpromptsvc.Prompt
	Content campoOmitido `json:"content,omitempty"`
}

func proyectarPolicySlim(p *projectpolicysvc.Policy) policySlim {
	if p == nil {
		return policySlim{}
	}
	return policySlim{Policy: *p, BodyMDLen: len(p.BodyMD)}
}

func proyectarPoliciesParaListado(list []*projectpolicysvc.Policy) []policySlim {
	items := make([]policySlim, 0, len(list))
	for _, p := range list {
		if p == nil {
			continue
		}
		items = append(items, proyectarPolicySlim(p))
	}
	return items
}

func proyectarPromptsCapturadosParaListado(list []*capturedpromptsvc.Prompt) []promptCapturadoSlim {
	items := make([]promptCapturadoSlim, 0, len(list))
	for _, p := range list {
		if p == nil {
			continue
		}
		items = append(items, promptCapturadoSlim{Prompt: *p})
	}
	return items
}

func proyectarSkillSlim(s *skillsvc.Skill) skillSlim {
	if s == nil {
		return skillSlim{}
	}
	return skillSlim{Skill: *s, ContentLen: len(s.Content)}
}

func proyectarAgentSlim(a *agentsvc.Agent) agentSlim {
	if a == nil {
		return agentSlim{}
	}
	return agentSlim{Agent: *a, SystemPromptLen: len(a.SystemPrompt)}
}

func proyectarFlowSlim(f *flowsvc.Flow) flowSlim {
	if f == nil {
		return flowSlim{}
	}
	return flowSlim{Flow: *f, SpecVersion: f.Spec.Version, SpecStepsCount: len(f.Spec.Steps)}
}

func proyectarSkillsParaListado(list []skillsvc.Skill) []skillSlim {
	items := make([]skillSlim, 0, len(list))
	for i := range list {
		items = append(items, proyectarSkillSlim(&list[i]))
	}
	return items
}

func proyectarSkillSearchParaListado(list []skillsvc.SearchResult) []skillSearchSlim {
	items := make([]skillSearchSlim, 0, len(list))
	for i := range list {
		items = append(items, skillSearchSlim{SearchResult: list[i], ContentLen: len(list[i].Content)})
	}
	return items
}

func proyectarAgentsParaListado(list []agentsvc.Agent) []agentSlim {
	items := make([]agentSlim, 0, len(list))
	for i := range list {
		items = append(items, proyectarAgentSlim(&list[i]))
	}
	return items
}

func proyectarFlowsParaListado(list []flowsvc.Flow) []flowSlim {
	items := make([]flowSlim, 0, len(list))
	for i := range list {
		items = append(items, proyectarFlowSlim(&list[i]))
	}
	return items
}
