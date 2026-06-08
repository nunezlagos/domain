package anonymizer

import (
	"encoding/json"
	"strings"
)

// RedactJSON walk recursivo enmascarando valores cuyas keys matchean
// la lista sensitive (match exacto, lowercase). Si una key termina en
// `_email`, `_rut`, `_phone`, `_password`, `_secret`, `_token` también se enmascara.
//
// Retorna nil si la entrada no es JSON válido (caller mantiene el original).
func RedactJSON(raw []byte, sensitive []string) []byte {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	set := make(map[string]struct{}, len(sensitive))
	for _, k := range sensitive {
		set[strings.ToLower(k)] = struct{}{}
	}
	masked := walkRedact(v, set)
	out, err := json.Marshal(masked)
	if err != nil {
		return raw
	}
	return out
}

func walkRedact(v any, sensitive map[string]struct{}) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if isSensitiveKey(k, sensitive) {
				out[k] = "[REDACTED]"
				continue
			}
			out[k] = walkRedact(val, sensitive)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = walkRedact(e, sensitive)
		}
		return out
	default:
		return v
	}
}

var sensitiveSuffixes = []string{"_email", "_rut", "_phone", "_password", "_secret", "_token", "_apikey"}

func isSensitiveKey(k string, set map[string]struct{}) bool {
	lk := strings.ToLower(k)
	if _, ok := set[lk]; ok {
		return true
	}
	for _, suf := range sensitiveSuffixes {
		if strings.HasSuffix(lk, suf) {
			return true
		}
	}
	return false
}
