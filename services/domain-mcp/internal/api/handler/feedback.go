package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	feedbacksvc "nunezlagos/domain/internal/service/feedback"
)

// feedbackCreateReq es el body del POST /api/v1/feedback.
// El Django (domain-admin) lo manda al hacer click en 👍/👎 debajo de una
// respuesta del assistant. message_id + skill_slug salen del mensaje del chat.
type feedbackCreateReq struct {
	MessageID int64  `json:"message_id"`
	Rating    int    `json:"rating"`
	Comment   string `json:"comment"`
	SkillSlug string `json:"skill_slug"`
	UserEmail string `json:"user_email"`
}

// createFeedback maneja POST /api/v1/feedback.
//
// CSRF-exempt: es una API REST con Bearer auth (no usa cookies de sesion del
// Django). Rate limit 30/min por user_email (anti-spam) aplicado antes de tocar
// la DB. Idempotente por message_id (upsert en el service).
func (a *API) createFeedback(w http.ResponseWriter, r *http.Request) {
	if a.Feedback == nil {
		writeError(w, http.StatusServiceUnavailable, "feedback_disabled", "")
		return
	}

	var in feedbackCreateReq
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "body invalido")
		return
	}

	// Rate limit por user_email (fallback a IP si no viene email).
	if a.FeedbackLimiter != nil {
		key := strings.ToLower(strings.TrimSpace(in.UserEmail))
		if key == "" {
			key = realIP(r)
		}
		if !a.FeedbackLimiter.Allow("feedback:" + key) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "demasiados feedbacks; intenta en un minuto")
			return
		}
	}

	f, err := a.Feedback.Create(r.Context(), feedbacksvc.UpsertParams{
		MessageID: in.MessageID,
		SkillSlug: in.SkillSlug,
		Rating:    in.Rating,
		Comment:   in.Comment,
		UserEmail: in.UserEmail,
	})
	switch {
	case errors.Is(err, feedbacksvc.ErrInvalidRating):
		writeError(w, http.StatusBadRequest, "invalid_rating", "rating debe ser 1 o -1")
		return
	case errors.Is(err, feedbacksvc.ErrInvalidMessage):
		writeError(w, http.StatusBadRequest, "invalid_message", "message_id requerido")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}

	writeData(w, http.StatusOK, f)
}

// listFeedback maneja GET /api/v1/feedback?skill_slug=&days=&rating=&limit=&offset=.
//
// Si days>0 devuelve los agregados por dia (count_up/count_down) — vista de
// metricas. Si no, devuelve la lista paginada de feedback crudo (vista admin).
func (a *API) listFeedback(w http.ResponseWriter, r *http.Request) {
	if a.Feedback == nil {
		writeError(w, http.StatusServiceUnavailable, "feedback_disabled", "")
		return
	}

	q := r.URL.Query()
	skillSlug := strings.TrimSpace(q.Get("skill_slug"))

	if daysStr := q.Get("days"); daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err != nil || days <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_days", "days debe ser un entero positivo")
			return
		}
		aggs, err := a.Feedback.AggregateByDay(r.Context(), days)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "aggregate_failed", err.Error())
			return
		}
		// Si pidieron un skill_slug puntual, filtrar el resultado in-memory.
		if skillSlug != "" {
			filtered := aggs[:0]
			for _, agg := range aggs {
				if agg.SkillSlug == skillSlug {
					filtered = append(filtered, agg)
				}
			}
			aggs = filtered
		}
		writeData(w, http.StatusOK, aggs)
		return
	}

	rating := parseRatingFilter(q.Get("rating"))
	limit := parseIntDefault(q.Get("limit"), 50)
	offset := parseIntDefault(q.Get("offset"), 0)

	items, total, err := a.Feedback.ListBySkill(r.Context(), feedbacksvc.ListFilter{
		SkillSlug: skillSlug,
		Rating:    rating,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": ensureJSONSlice(items),
		"meta": map[string]any{"total": total, "limit": limit, "offset": offset},
	})
}

// parseRatingFilter normaliza el query param rating a 0 (sin filtro), 1 o -1.
// Acepta "positive"/"negative" ademas de los numericos.
func parseRatingFilter(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "positive", "up":
		return 1
	case "-1", "negative", "down":
		return -1
	default:
		return 0
	}
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
