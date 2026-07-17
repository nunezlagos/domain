package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	skillsvc "nunezlagos/domain/internal/service/skill"
)

// Secciones del prepared_context (DOMAINSERV-38). Cada helper es best-effort:
// registra su resultado en la métrica OrchestratorContextPrepSectionsTotal y
// nunca aborta la fase. section={policies|skills|obs}, result={ok|empty|no_service|error}.

// recordPrepSection incrementa el contador low-cardinality de observabilidad.
func (s *Service) recordPrepSection(section, result string) {
	if s.Metrics != nil {
		s.Metrics.OrchestratorContextPrepSectionsTotal.WithLabelValues(section, result).Inc()
	}
}

// prepPolicies inyecta las policies activas del proyecto.
func (s *Service) prepPolicies(ctx context.Context, orgID, projectID uuid.UUID, b *strings.Builder) {
	if s.ProjectPolicies == nil {
		s.recordPrepSection("policies", "no_service")
		return
	}
	pols, err := s.ProjectPolicies.List(ctx, orgID, projectID, "")
	if err != nil {
		s.recordPrepSection("policies", "error")
		return
	}
	var sb strings.Builder
	n := 0
	for _, p := range pols {
		if !p.IsActive {
			continue
		}
		fmt.Fprintf(&sb, "- **%s** (%s): %s\n", p.Name, p.Kind, firstLine(p.BodyMD))
		if n++; n >= prepMaxPolicies {
			break
		}
	}
	if n == 0 {
		s.recordPrepSection("policies", "empty")
		return
	}
	fmt.Fprintln(b, "### Policies del proyecto (vigentes)")
	b.WriteString(sb.String())
	fmt.Fprintln(b)
	s.recordPrepSection("policies", "ok")
}

// prepSkills inyecta las skills relevantes y APLICABLES al proyecto usando
// SearchHybrid + ApplicableSkillIDs (globals + project, sin excluidas). Reemplaza
// el List() ciego anterior que ignoraba scoping y relevancia (DOMAINSERV-38).
func (s *Service) prepSkills(ctx context.Context, orgID, projectID uuid.UUID, slug string, b *strings.Builder) {
	if s.Skills == nil {
		s.recordPrepSection("skills", "no_service")
		return
	}
	query := fmt.Sprintf("skills for %s phase", slug)
	results, err := s.Skills.SearchHybrid(ctx, orgID, query, prepSkillCandidates)
	if err != nil {
		s.recordPrepSection("skills", "error")
		return
	}
	applicable, err := s.Skills.ApplicableSkillIDs(ctx, projectID)
	if err != nil {
		s.recordPrepSection("skills", "error")
		return
	}
	lines, n := renderSkillLines(results, applicable)
	if n == 0 {
		s.recordPrepSection("skills", "empty")
		return
	}
	fmt.Fprintln(b, skillsSectionHeader)
	b.WriteString(lines)
	fmt.Fprintln(b)
	s.recordPrepSection("skills", "ok")
}

// renderSkillLines arma los bullets de skills aplicables (globals + project, sin
// excluidas), hasta prepMaxSkills. Devuelve el bloque y cuántas se incluyeron.
func renderSkillLines(results []skillsvc.SearchResult, applicable map[uuid.UUID]bool) (string, int) {
	var sb strings.Builder
	n := 0
	for _, r := range results {
		if applicable != nil && !applicable[r.ID] {
			continue // skill de otro proyecto o excluida
		}
		fmt.Fprintf(&sb, "- **%s**: %s\n", r.Name, truncate(r.Description, prepSkillBodyMax))
		if n++; n >= prepMaxSkills {
			break
		}
	}
	return sb.String(), n
}

// prepObs inyecta las observaciones recientes del proyecto.
func (s *Service) prepObs(ctx context.Context, projectID uuid.UUID, b *strings.Builder) {
	if s.Observations == nil {
		s.recordPrepSection("obs", "no_service")
		return
	}
	obs, err := s.Observations.List(ctx, projectID, prepMaxObs)
	if err != nil {
		s.recordPrepSection("obs", "error")
		return
	}
	if len(obs) == 0 {
		s.recordPrepSection("obs", "empty")
		return
	}
	fmt.Fprintln(b, "### Contexto reciente (memoria)")
	for _, o := range obs {
		fmt.Fprintf(b, "- [%s] %s\n", o.ObservationType, firstLine(o.Content))
	}
	fmt.Fprintln(b)
	s.recordPrepSection("obs", "ok")
}

// firstLine devuelve la primera línea no vacía de s, recortada, para resúmenes
// de una línea en el bloque de contexto.
func firstLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			return truncate(ln, 160)
		}
	}
	return ""
}

// truncate recorta s a max caracteres con elipsis, colapsando saltos de línea
// para que el resumen quede en una sola línea del bloque markdown. Recorta por
// runes (no bytes) para no partir caracteres UTF-8 multibyte (tildes, ñ, —).
func truncate(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > max {
		return string(r[:max-3]) + "..."
	}
	return s
}
