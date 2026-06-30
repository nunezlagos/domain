package skill_suggestions

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/skill_suggestions/skillsuggestionsdb"
)

// Aggregator es el batch del judge (HU-52.3): itera skills activos, arma el
// contexto de cada uno (skill + senales 30d + top-3 similares lexicos), pide
// sugerencias al LLMJudge y las persiste como 'pending' via Service.Create.
//
// SEPARACION FISICA (regla dura 6): el Aggregator SOLO llama Service.Create
// (inserta pending). JAMAS llama Apply. El Apply muta `skills` y corre
// exclusivamente por accion humana (approve+apply). Esta garantia esta cubierta
// por test (no hay ninguna ruta del cron que llegue a Apply).
//
// DEGRADACION (regla dura 7): si el judge no esta disponible (sin MINIMAX_API_KEY)
// Run devuelve ErrJudgeUnavailable sin tocar la DB. El cron lo loguea limpio.
type Aggregator struct {
	Pool    *pgxpool.Pool
	Service *Service
	Judge   *LLMJudge

	// MaxSkills acota cuantos skills se escanean por corrida (rate_limit del
	// escaneo). 0 => default 200. El total de sugerencias persistidas ademas se
	// corta en MaxBatch (50) globalmente.
	MaxSkills int

	// SimilarLimit cuantos similares lexicos se pasan al judge (default 3).
	SimilarLimit int

	Logger *slog.Logger
}

// RunResult resume una corrida del aggregator (para logging/CLI).
type RunResult struct {
	SkillsScanned   int `json:"skills_scanned"`    // skills evaluados
	Suggestions     int `json:"suggestions"`       // sugerencias validas devueltas por el judge
	Persisted       int `json:"persisted"`         // efectivamente insertadas (no deduplicadas)
	Deduped         int `json:"deduped"`           // descartadas por dedup (UNIQUE parcial)
	SkillsWithError int `json:"skills_with_error"` // skills cuya evaluacion fallo (se loguea y se sigue)
}

// defaults aplica los valores por defecto in-place.
func (a *Aggregator) defaults() {
	if a.MaxSkills <= 0 {
		a.MaxSkills = 200
	}
	if a.SimilarLimit <= 0 {
		a.SimilarLimit = 3
	}
	if a.Logger == nil {
		a.Logger = slog.Default()
	}
}

// Run ejecuta una pasada del judge sobre los skills activos. Devuelve un resumen.
// Degrada con ErrJudgeUnavailable si no hay LLM (sin tocar la DB).
func (a *Aggregator) Run(ctx context.Context) (RunResult, error) {
	a.defaults()
	var res RunResult

	if a.Service == nil || a.Pool == nil {
		return res, fmt.Errorf("aggregator: Service/Pool no inyectado")
	}
	if a.Judge == nil || !a.Judge.Available() {
		// Regla dura 7: sin LLM no se corre. No es un crash.
		return res, ErrJudgeUnavailable
	}

	q := skillsuggestionsdb.New(a.Pool)
	skills, err := q.JudgeListActiveSkills(ctx, skillsuggestionsdb.JudgeListActiveSkillsParams{
		ResultLimit:  int32(a.MaxSkills),
		ResultOffset: 0,
	})
	if err != nil {
		return res, fmt.Errorf("listar skills activos: %w", err)
	}

	for _, sk := range skills {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		// Rate_limit global: no persistir mas de MaxBatch por corrida.
		if res.Persisted >= MaxBatch {
			a.Logger.Info("skill-judge: tope de batch alcanzado", slog.Int("max", MaxBatch))
			break
		}

		in, berr := a.buildInput(ctx, q, sk)
		if berr != nil {
			res.SkillsWithError++
			a.Logger.Error("skill-judge: armado de contexto fallo",
				slog.String("slug", sk.Slug), slog.Any("err", berr))
			continue
		}
		res.SkillsScanned++

		suggestions, eerr := a.Judge.Evaluate(ctx, in)
		if eerr != nil {
			res.SkillsWithError++
			a.Logger.Error("skill-judge: evaluacion LLM fallo",
				slog.String("slug", sk.Slug), slog.Any("err", eerr))
			continue
		}

		for _, ci := range suggestions {
			if res.Persisted >= MaxBatch {
				break
			}
			res.Suggestions++
			created, cerr := a.Service.Create(ctx, ci)
			if cerr != nil {
				res.SkillsWithError++
				a.Logger.Error("skill-judge: persistir sugerencia fallo",
					slog.String("slug", ci.SkillSlug), slog.String("kind", ci.Kind),
					slog.Any("err", cerr))
				continue
			}
			if created == nil {
				// Dedup: ya habia una pendiente identica (skill_slug, kind).
				res.Deduped++
				continue
			}
			res.Persisted++
		}
	}

	a.Logger.Info("skill-judge: pasada completa",
		slog.Int("skills_scanned", res.SkillsScanned),
		slog.Int("suggestions", res.Suggestions),
		slog.Int("persisted", res.Persisted),
		slog.Int("deduped", res.Deduped),
		slog.Int("skills_with_error", res.SkillsWithError))
	return res, nil
}

// buildInput arma el SkillInput de un skill: senales 30d (metrics + feedback +
// last-use) + top-3 similares lexicos. Todo se lee de la DB aca; el judge
// (Evaluate) solo razona, no toca la DB (testeable).
func (a *Aggregator) buildInput(ctx context.Context, q *skillsuggestionsdb.Queries, sk skillsuggestionsdb.JudgeListActiveSkillsRow) (SkillInput, error) {
	slug := sk.Slug
	sig, err := q.JudgeSkillSignals(ctx, skillsuggestionsdb.JudgeSkillSignalsParams{
		MetricsSkillID: sk.ID,
		SkillSlug:      &slug,
		ExecSkillID:    sk.ID,
	})
	if err != nil {
		return SkillInput{}, fmt.Errorf("senales del skill: %w", err)
	}

	in := SkillInput{
		Slug:               sk.Slug,
		Name:               sk.Name,
		Description:        sk.Description,
		Content:            sk.Content,
		SeedManaged:        sk.SeedManaged,
		InvocationsPerDay:  float64(sig.Invocations30d) / 30.0,
		FailureRate:        failureRate(sig.Invocations30d, sig.Failures30d),
		AvgDurationSeconds: avgDurationSeconds(sig.Invocations30d, sig.DurationWeighted30d),
		NegativeFeedback:   int(sig.NegativeFeedback30d),
		DaysSinceLastUse:   int(sig.DaysSinceLastUse),
	}

	// Top-N similares por similitud lexica (description_tsv), excluyendo el propio.
	queryText := strings.TrimSpace(sk.Name + " " + sk.Description)
	if queryText != "" {
		sims, serr := q.JudgeSimilarSkills(ctx, skillsuggestionsdb.JudgeSimilarSkillsParams{
			QueryText:   queryText,
			SelfID:      sk.ID,
			ResultLimit: int32(a.SimilarLimit),
		})
		if serr != nil {
			return SkillInput{}, fmt.Errorf("similares lexicos: %w", serr)
		}
		for _, s := range sims {
			in.Similar = append(in.Similar, SimilarSkill{Slug: s.Slug, Name: s.Name, Score: s.Score})
		}
	}
	return in, nil
}

// failureRate computa el % de fallos 30d (0 si no hubo invocaciones).
func failureRate(invocations, failures int64) float64 {
	if invocations <= 0 {
		return 0
	}
	return float64(failures) / float64(invocations) * 100.0
}

// avgDurationSeconds desnormaliza el promedio ponderado (sum(avg_ms*invs)) a
// segundos por invocacion (0 si no hubo invocaciones).
func avgDurationSeconds(invocations, durationWeighted int64) float64 {
	if invocations <= 0 {
		return 0
	}
	avgMs := float64(durationWeighted) / float64(invocations)
	return avgMs / 1000.0
}
