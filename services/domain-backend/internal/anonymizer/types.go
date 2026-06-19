// Package anonymizer — issue-25.11 staging dump anonymization.
//
// Pipeline: source pool (prod-mirror) → transform per table+col → dest pool
// (staging). Idempotente dado el mismo seed (lookup table + idx determinístico).
package anonymizer

// Rule define cómo se transforma una columna durante el dump.
type Rule string

const (
	// RulePassthrough: copia tal cual.
	RulePassthrough Rule = "passthrough"
	// RuleFakerEmail: reemplaza con `user{idx}@example.test`.
	RuleFakerEmail Rule = "faker_email"
	// RuleFakerName: reemplaza con nombre ficticio determinístico.
	RuleFakerName Rule = "faker_name"
	// RuleFakerRUT: reemplaza con RUT chileno secuencial válido (módulo 11).
	RuleFakerRUT Rule = "faker_rut"
	// RuleFakerPhone: reemplaza con `+5690000{idx:04d}`.
	RuleFakerPhone Rule = "faker_phone"
	// RuleRedactContent: reemplaza con `[REDACTED-{sha256[:16]}]`.
	RuleRedactContent Rule = "redact_content"
	// RuleNullify: setea NULL.
	RuleNullify Rule = "nullify"
	// RuleJSONRedact: walk JSONB enmascarando keys sensibles.
	RuleJSONRedact Rule = "json_redact"
)

// TableConfig declara la política para una tabla.
type TableConfig struct {
	// Skip: si true, no se copia la tabla al destino (los registros se omiten).
	Skip bool
	// Columns: regla por columna. Las no listadas son passthrough.
	Columns map[string]Rule
}

// Config describe el dump anonimizado completo.
type Config struct {
	// Seed determinístico para los fakers.
	Seed int64
	// Tables: política por tabla. Tablas no listadas son passthrough sin transform.
	Tables map[string]TableConfig
	// SensitiveJSONKeys: lista de keys que se enmascaran cuando RuleJSONRedact aplica.
	SensitiveJSONKeys []string
}

// DefaultSensitiveJSONKeys son las claves redactadas por defecto en JSONB.
var DefaultSensitiveJSONKeys = []string{
	"email", "rut", "phone", "password", "secret",
	"api_key", "apikey", "token", "card_number", "cvc",
}

// DefaultConfig retorna la política recomendada para Domain.
// Cubre las tablas con PII identificadas en .claude/rules/security.md.
func DefaultConfig() Config {
	return Config{
		Seed:              1,
		SensitiveJSONKeys: DefaultSensitiveJSONKeys,
		Tables: map[string]TableConfig{
			"users": {
				Columns: map[string]Rule{
					"email": RuleFakerEmail,
					"rut":   RuleFakerRUT,
					"name":  RuleFakerName,
					"phone": RuleFakerPhone,
				},
			},
			"organizations": {
				Columns: map[string]Rule{
					"name": RuleFakerName,
				},
			},
			"knowledge_observations": {
				Columns: map[string]Rule{
					"content": RuleRedactContent,
				},
			},
			"audit_log": {
				Columns: map[string]Rule{
					"diff":       RuleJSONRedact,
					"new_values": RuleJSONRedact,
					"old_values": RuleJSONRedact,
				},
			},
			"auth_api_keys":                  {Skip: true},
			"auth_otp_codes":                 {Skip: true},
			"webhook_outbound_subscriptions": {Columns: map[string]Rule{"secret_cipher": RuleNullify}},
		},
	}
}
