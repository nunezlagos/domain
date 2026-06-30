package skill_suggestions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/skill_suggestions/skillsuggestionsdb"
	"nunezlagos/domain/internal/store/txctx"
)

// ---- payloads tipados (forma segun kind, ver mig 000182) ----

type splitChild struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Instruction string `json:"instruction"` // si Content vacio, generar via LLM
}

type splitPayload struct {
	Children []splitChild `json:"children"`
}

type mergePayload struct {
	With          []string `json:"with"`        // slugs a consolidar (ademas del skill_slug)
	MergedSlug    string   `json:"merged_slug"` // slug del skill consolidado
	MergedName    string   `json:"merged_name"`
	MergedContent string   `json:"merged_content"`
}

type refinePayload struct {
	NewContent  string `json:"new_content"`
	Instruction string `json:"instruction"` // si NewContent vacio, generar via LLM
	Changelog   string `json:"changelog"`
}

type archivePayload struct {
	Reason string `json:"reason"`
}

// Apply ejecuta la sugerencia aprobada sobre `skills`. SOLO accion humana
// (el cron jamas la llama). Pasos:
//  1. Cargar la sugerencia y validar que este 'approved' (no pending/applied).
//  2. Abrir UNA transaccion; inyectarla en el ctx (txctx) para que TODAS las
//     queries (skills + skill_versions + el mark-applied) sean atomicas.
//  3. Despachar por kind. Guards: seed_managed nunca archive/split.
//  4. Marcar applied (guard optimista status='approved' AND applied_at IS NULL).
//  5. Commit. Si algo falla -> rollback: la sugerencia queda 'approved'
//     applied_at=NULL (reintentable). Audit 'applied' (ok) o 'apply_failed'.
func (s *Service) Apply(ctx context.Context, id uuid.UUID, reviewer *uuid.UUID) (*Suggestion, *ApplyResult, error) {
	sug, err := s.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	switch sug.Status {
	case StatusApplied:
		return nil, nil, ErrAlreadyApplied
	case StatusApproved:
		// ok, aplicable
	default:
		return nil, nil, ErrNotApproved
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin apply tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op si ya hubo Commit

	txCtx := txctx.WithTxContext(ctx, tx)
	q := skillsuggestionsdb.New(tx)

	result, err := s.dispatchApply(txCtx, q, sug)
	if err != nil {
		// Rollback (defer) deja la sugerencia 'approved' applied_at=NULL.
		s.audit(ctx, audit.Event{
			ActorType:  audit.ActorUser,
			ActorID:    reviewer,
			Action:     "skill_suggestion.apply_failed",
			EntityType: "skill_suggestion",
			EntityID:   &sug.ID,
			NewValues:  map[string]any{"skill_slug": sug.SkillSlug, "kind": sug.Kind, "error": err.Error()},
		})
		return nil, nil, err
	}

	changesJSON, _ := json.Marshal(result)
	applied, err := q.SuggestionMarkApplied(txCtx, skillsuggestionsdb.SuggestionMarkAppliedParams{
		ID:             sug.ID,
		AppliedChanges: changesJSON,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Concurrencia: otro reviewer aplico primero. Rollback y reportar.
		return nil, nil, ErrAlreadyApplied
	}
	if err != nil {
		return nil, nil, fmt.Errorf("mark applied: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit apply: %w", err)
	}

	out := suggestionFromMarkApplied(applied)
	s.audit(ctx, audit.Event{
		ActorType:  audit.ActorUser,
		ActorID:    reviewer,
		Action:     "skill_suggestion.applied",
		EntityType: "skill_suggestion",
		EntityID:   &out.ID,
		OldValues:  map[string]any{"status": StatusApproved},
		NewValues:  auditValues(out, changesJSON),
	})
	return out, result, nil
}

// dispatchApply ejecuta la mutacion segun kind dentro de la tx ya abierta.
func (s *Service) dispatchApply(ctx context.Context, q *skillsuggestionsdb.Queries, sug *Suggestion) (*ApplyResult, error) {
	switch sug.Kind {
	case KindArchive:
		return s.applyArchive(ctx, q, sug)
	case KindRefine:
		return s.applyRefine(ctx, q, sug)
	case KindSplit:
		return s.applySplit(ctx, q, sug)
	case KindMerge:
		return s.applyMerge(ctx, q, sug)
	default:
		return nil, ErrInvalidKind
	}
}

// applyArchive: soft-delete del skill (jamas seed_managed). 0 filas = ya borrado
// o seed_managed (guard de la query) -> error claro.
func (s *Service) applyArchive(ctx context.Context, q *skillsuggestionsdb.Queries, sug *Suggestion) (*ApplyResult, error) {
	sk, err := s.resolveSkill(ctx, q, sug.SkillSlug)
	if err != nil {
		return nil, err
	}
	if sk.SeedManaged {
		return nil, ErrSeedManaged
	}
	n, err := q.SuggestionSkillArchive(ctx, sk.ID)
	if err != nil {
		return nil, fmt.Errorf("archive skill: %w", err)
	}
	if n == 0 {
		return nil, ErrSeedManaged // guard de la query (seed o ya borrado)
	}
	return &ApplyResult{Kind: KindArchive, ArchivedSlug: sug.SkillSlug}, nil
}

// applyRefine: snapshot de la version actual (reversible) + UPDATE content. Si
// el payload no trae new_content, lo genera via Refiner (LLM). Sin Refiner y sin
// content -> ErrApplyUnavailable (degradacion limpia).
func (s *Service) applyRefine(ctx context.Context, q *skillsuggestionsdb.Queries, sug *Suggestion) (*ApplyResult, error) {
	var p refinePayload
	if err := json.Unmarshal(sug.Payload, &p); err != nil {
		return nil, fmt.Errorf("refine payload invalido: %w", err)
	}
	sk, err := s.resolveSkill(ctx, q, sug.SkillSlug)
	if err != nil {
		return nil, err
	}

	newContent := strings.TrimSpace(p.NewContent)
	if newContent == "" {
		if s.Refiner == nil {
			return nil, ErrApplyUnavailable
		}
		gen, gerr := s.Refiner.RefineContent(ctx, sug.SkillSlug, sk.Content, p.Instruction)
		if gerr != nil {
			return nil, fmt.Errorf("refine via LLM: %w", gerr)
		}
		newContent = strings.TrimSpace(gen)
		if newContent == "" {
			return nil, ErrApplyUnavailable
		}
	}

	// Snapshot del content ACTUAL antes de pisar (reversible). Honra la tx.
	res := &ApplyResult{Kind: KindRefine}
	if s.Versions != nil {
		changelog := p.Changelog
		if changelog == "" {
			changelog = "refine via skill judge (HU-52.3)"
		}
		cur := sk.Content
		v, verr := s.Versions.RecordVersion(ctx, sk.ID, &cur, &changelog, nil)
		if verr != nil {
			return nil, fmt.Errorf("snapshot version: %w", verr)
		}
		res.RefineVersion = &v
	}

	n, err := q.SuggestionSkillRefineContent(ctx, skillsuggestionsdb.SuggestionSkillRefineContentParams{
		ID:      sk.ID,
		Content: &newContent,
	})
	if err != nil {
		return nil, fmt.Errorf("refine content: %w", err)
	}
	if n == 0 {
		return nil, ErrSkillNotFound
	}
	return res, nil
}

// applySplit: crea N hijos (parent_skill_id = original) y soft-delete el original
// (jamas seed_managed). Si un child no trae content, lo genera via Refiner.
func (s *Service) applySplit(ctx context.Context, q *skillsuggestionsdb.Queries, sug *Suggestion) (*ApplyResult, error) {
	var p splitPayload
	if err := json.Unmarshal(sug.Payload, &p); err != nil {
		return nil, fmt.Errorf("split payload invalido: %w", err)
	}
	if len(p.Children) < 2 {
		return nil, fmt.Errorf("split requiere >= 2 children")
	}
	sk, err := s.resolveSkill(ctx, q, sug.SkillSlug)
	if err != nil {
		return nil, err
	}
	if sk.SeedManaged {
		return nil, ErrSeedManaged
	}

	res := &ApplyResult{Kind: KindSplit}
	for _, c := range p.Children {
		slug := strings.TrimSpace(c.Slug)
		if slug == "" {
			return nil, fmt.Errorf("split child sin slug")
		}
		content := strings.TrimSpace(c.Content)
		if content == "" {
			if s.Refiner == nil {
				return nil, ErrApplyUnavailable
			}
			gen, gerr := s.Refiner.RefineContent(ctx, slug, sk.Content, c.Instruction)
			if gerr != nil {
				return nil, fmt.Errorf("split child via LLM: %w", gerr)
			}
			content = strings.TrimSpace(gen)
		}
		var descPtr, contentPtr *string
		if c.Description != "" {
			d := c.Description
			descPtr = &d
		}
		if content != "" {
			contentPtr = &content
		}
		name := c.Name
		if name == "" {
			name = slug
		}
		parentID := sk.ID
		child, cerr := q.SuggestionSkillCreateChild(ctx, skillsuggestionsdb.SuggestionSkillCreateChildParams{
			Slug:          slug,
			Name:          name,
			Description:   descPtr,
			Content:       contentPtr,
			ParentSkillID: &parentID,
		})
		if cerr != nil {
			return nil, fmt.Errorf("crear child %q: %w", slug, cerr)
		}
		res.CreatedSkills = append(res.CreatedSkills, child.Slug)
	}

	n, err := q.SuggestionSkillMarkSplitParent(ctx, sk.ID)
	if err != nil {
		return nil, fmt.Errorf("soft-delete original split: %w", err)
	}
	if n == 0 {
		return nil, ErrSeedManaged
	}
	res.SupersededSlugs = []string{sug.SkillSlug}
	return res, nil
}

// applyMerge: crea (o reusa el slug propuesto) un skill consolidado y marca los
// originales (skill_slug + with[]) como superseded_by + soft-delete. NUNCA
// DELETE fisico (rompe metricas por FK CASCADE).
func (s *Service) applyMerge(ctx context.Context, q *skillsuggestionsdb.Queries, sug *Suggestion) (*ApplyResult, error) {
	var p mergePayload
	if err := json.Unmarshal(sug.Payload, &p); err != nil {
		return nil, fmt.Errorf("merge payload invalido: %w", err)
	}
	if len(p.With) == 0 {
		return nil, fmt.Errorf("merge requiere al menos otro skill en 'with'")
	}
	mergedSlug := strings.TrimSpace(p.MergedSlug)
	if mergedSlug == "" {
		return nil, fmt.Errorf("merge requiere merged_slug")
	}

	// El skill_slug actua como "padre" para heredar organization_id/skill_type.
	base, err := s.resolveSkill(ctx, q, sug.SkillSlug)
	if err != nil {
		return nil, err
	}
	if base.SeedManaged {
		return nil, ErrSeedManaged
	}

	var contentPtr *string
	if mc := strings.TrimSpace(p.MergedContent); mc != "" {
		contentPtr = &mc
	}
	name := p.MergedName
	if name == "" {
		name = mergedSlug
	}
	baseID := base.ID
	consolidated, cerr := q.SuggestionSkillCreateChild(ctx, skillsuggestionsdb.SuggestionSkillCreateChildParams{
		Slug:          mergedSlug,
		Name:          name,
		Description:   nil,
		Content:       contentPtr,
		ParentSkillID: &baseID,
	})
	if cerr != nil {
		return nil, fmt.Errorf("crear skill consolidado: %w", cerr)
	}

	res := &ApplyResult{Kind: KindMerge, CreatedSkills: []string{consolidated.Slug}}

	// Marcar TODOS los originales (skill_slug + with) como superseded.
	originals := append([]string{sug.SkillSlug}, p.With...)
	consolidatedID := consolidated.ID
	for _, slug := range dedupStrings(originals) {
		if slug == mergedSlug {
			continue // no superseder el consolidado por si mismo
		}
		orig, oerr := s.resolveSkill(ctx, q, slug)
		if oerr != nil {
			if errors.Is(oerr, ErrSkillNotFound) {
				continue // ya borrado: tolerar
			}
			return nil, oerr
		}
		if orig.SeedManaged {
			return nil, ErrSeedManaged // jamas superseder un seed (consistente con ARCHIVE/SPLIT)
		}
		n, serr := q.SuggestionSkillSupersede(ctx, skillsuggestionsdb.SuggestionSkillSupersedeParams{
			ID:           orig.ID,
			SupersededBy: &consolidatedID,
		})
		if serr != nil {
			return nil, fmt.Errorf("supersede %q: %w", slug, serr)
		}
		if n > 0 {
			res.SupersededSlugs = append(res.SupersededSlugs, slug)
		}
	}
	return res, nil
}

// resolveSkill carga el skill vivo por slug dentro de la tx, o ErrSkillNotFound.
func (s *Service) resolveSkill(ctx context.Context, q *skillsuggestionsdb.Queries, slug string) (*skillsuggestionsdb.SuggestionSkillBySlugRow, error) {
	row, err := q.SuggestionSkillBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSkillNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolve skill %q: %w", slug, err)
	}
	return &row, nil
}

func dedupStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := in[:0]
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
