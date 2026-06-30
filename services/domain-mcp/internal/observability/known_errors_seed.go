// Package observability: este archivo siembra known_errors canonicos del
// dominio. El operador extiende el catalogo via la tool domain_known_error_set
// sin redeploy.
//
// issue-53.9 early-error-reporting.
package observability

// KnownErrorSeeds devuelve los known_errors iniciales. Los fingerprints se
// derivan de (category, mensaje canonico, source) — el operador agrega mas
// a partir de los fingerprints reales que observe en error_events.
func KnownErrorSeeds() []KnownError {
	return []KnownError{
		{
			Fingerprint:    Fingerprint(CategoryTimeout, "context deadline exceeded", "db", ""),
			Name:           "db-context-deadline",
			Recoverable:    true,
			AutoHealAction: HealRetry,
		},
		{
			Fingerprint:    Fingerprint(CategoryExternal, "connection refused", "external", ""),
			Name:           "external-conn-refused",
			Recoverable:    true,
			AutoHealAction: HealRetry,
		},
		{
			Fingerprint:    Fingerprint(CategorySQL, `relation "code_index_files" does not exist`, "bootstrap", ""),
			Name:           "code-index-missing-table",
			Recoverable:    false,
			AutoHealAction: HealNone,
		},
	}
}
