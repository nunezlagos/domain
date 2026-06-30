// Package anonymizer — issue-25.11 staging dump anonymization.
//
// Pipeline: source pool (prod-mirror) → transform per table+col → dest pool
// (staging). Idempotente dado el mismo seed (lookup table + idx determinístico).
package anonymizer

// Rule define cómo se transforma una columna durante el dump.
type Rule string

const (

	RulePassthrough Rule = "passthrough"

	RuleFakerEmail Rule = "faker_email"

	RuleFakerName Rule = "faker_name"

	RuleFakerRUT Rule = "faker_rut"

	RuleFakerPhone Rule = "faker_phone"

	RuleRedactContent Rule = "redact_content"

	RuleNullify Rule = "nullify"

	RuleJSONRedact Rule = "json_redact"
)

// TableConfig declara la política para una tabla.
type TableConfig struct {

	Skip bool

	Columns map[string]Rule
}

// Config describe el dump anonimizado completo.
type Config struct {

	Seed int64

	Tables map[string]TableConfig

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
			"webhook_outbound_subscriptions": {Columns: map[string]Rule{"secret_cipher": RuleNullify}},
		},
	}
}
