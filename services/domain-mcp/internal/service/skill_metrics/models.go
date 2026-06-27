// Package skill_metrics — HU-52.2: skill success rate tracking automatico.
//
// Backend puro (SIN UI). Repository + Service pattern, igual que feedback/
// observation. El Service contiene la logica de negocio (definicion de exito/
// fallo, guards de data insuficiente, division por cero) y delega persistencia
// a Repository. La implementacion concreta (pg_repository.go) honra la
// tx-context (issue-25.14).
//
// Single-tenant (regla dura 1): NADA de organization_id. El aislamiento natural
// es por skill_id (skills(id)).
//
// SELF-CONTAINED respecto a skill_feedback (HU-52.1): este paquete agrega
// skill_executions (ejecuciones reales del skill), no votos de usuarios.
package skill_metrics

import (
	"time"

	"github.com/google/uuid"
)

// SuccessRateThreshold es el umbral (porcentaje) por debajo del cual un skill
// se considera "fallando" para el hook de alertas (success_rate < 70%).
const SuccessRateThreshold = 70.0

// AlertMetricKey es el metric que matchea contra usage_alerts.metric para el
// hook de alertas (skill con baja tasa de exito por 3 dias seguidos).
const AlertMetricKey = "skill_success_rate"

// LowRateStreakDays son los dias consecutivos por debajo del umbral que disparan
// la alerta.
const LowRateStreakDays = 3

// DefaultDailyRetentionDays — cuanto retiene la tabla daily antes de cleanup.
const DefaultDailyRetentionDays = 90

// DefaultWeeklyRetentionDays — cuanto retiene la tabla weekly antes de cleanup.
const DefaultWeeklyRetentionDays = 365

// MinInvocationsForP95 — por debajo de esto NO se computa p95 (data insuficiente).
const MinInvocationsForP95 = 10

// DailyMetric es el agregado de un skill en un dia.
//   - SuccessRate/AvgDurationMs/P95DurationMs son punteros: nil = NULL (sin datos
//     suficientes). p95 es nil si invocations < 10.
type DailyMetric struct {
	SkillID            uuid.UUID  `json:"skill_id"`
	Day                time.Time  `json:"day"`
	InvocationsCount   int        `json:"invocations_count"`
	SuccessCount       int        `json:"success_count"`
	FailureCount       int        `json:"failure_count"`
	SuccessRate        *float64   `json:"success_rate,omitempty"`
	AvgDurationMs      *int       `json:"avg_duration_ms,omitempty"`
	P95DurationMs      *int       `json:"p95_duration_ms,omitempty"`
	UniqueCallersCount int        `json:"unique_callers_count"`
	CreatedAt          *time.Time `json:"created_at,omitempty"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
}

// TopFailed es una fila del ranking "peor success_rate" en una ventana.
type TopFailed struct {
	SkillID          uuid.UUID `json:"skill_id"`
	InvocationsCount int64     `json:"invocations_count"`
	SuccessCount     int64     `json:"success_count"`
	FailureCount     int64     `json:"failure_count"`
	SuccessRate      *float64  `json:"success_rate,omitempty"`
}

// Slowest es una fila del ranking "peor p95" en una ventana.
type Slowest struct {
	SkillID          uuid.UUID `json:"skill_id"`
	P95DurationMs    int       `json:"p95_duration_ms"`
	InvocationsCount int64     `json:"invocations_count"`
}

// LowRateStreak es un skill con 3 dias consecutivos por debajo del umbral.
type LowRateStreak struct {
	SkillID        uuid.UUID `json:"skill_id"`
	AvgSuccessRate float64   `json:"avg_success_rate"`
	StreakStart    time.Time `json:"streak_start"`
	StreakEnd      time.Time `json:"streak_end"`
}

// AggregateResult es el resultado computado de agregar un dia (antes de persistir).
type AggregateResult struct {
	InvocationsCount   int
	SuccessCount       int
	FailureCount       int
	SuccessRate        *float64
	AvgDurationMs      *int
	P95DurationMs      *int
	UniqueCallersCount int
}
