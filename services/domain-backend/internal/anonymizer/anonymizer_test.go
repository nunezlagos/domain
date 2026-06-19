package anonymizer

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestFakerEmail_Determinístico(t *testing.T) {
	a := FakerEmail(42, 5)
	b := FakerEmail(42, 5)
	if a != b {
		t.Fatalf("same input → diff output: %s vs %s", a, b)
	}
	if !strings.Contains(a, "@example.test") {
		t.Fatalf("missing test domain: %s", a)
	}
}

func TestFakerEmail_DistintoSeed(t *testing.T) {
	if FakerEmail(1, 0) == FakerEmail(2, 0) {
		t.Fatal("different seeds should differ")
	}
}

func TestFakerName_FormatoNombreApellido(t *testing.T) {
	name := FakerName(1, 0)
	parts := strings.Split(name, " ")
	if len(parts) != 2 {
		t.Fatalf("expected first+last, got %q", name)
	}
}

func TestFakerRUT_DVValido(t *testing.T) {
	for i := 0; i < 50; i++ {
		rut := FakerRUT(1, i)
		parts := strings.Split(rut, "-")
		if len(parts) != 2 {
			t.Fatalf("malformed rut: %s", rut)
		}
		base, err := strconv.Atoi(parts[0])
		if err != nil {
			t.Fatalf("base not int: %s", parts[0])
		}
		dv := rutDV(base)
		if dv != parts[1] {
			t.Fatalf("RUT %s claims DV %s but compute gives %s", rut, parts[1], dv)
		}
	}
}

func TestRedactJSON_EnmascaraKeysSensibles(t *testing.T) {
	in := []byte(`{"email":"x@y.z","name":"Ana","nested":{"phone":"123","tag":"ok"}}`)
	out := RedactJSON(in, DefaultSensitiveJSONKeys)
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatal(err)
	}
	if v["email"] != "[REDACTED]" {
		t.Fatalf("email not redacted: %v", v["email"])
	}
	if v["name"] != "Ana" {
		t.Fatalf("name should not be redacted: %v", v["name"])
	}
	nested, _ := v["nested"].(map[string]any)
	if nested["phone"] != "[REDACTED]" {
		t.Fatalf("nested.phone not redacted: %v", nested["phone"])
	}
	if nested["tag"] != "ok" {
		t.Fatalf("nested.tag should not be redacted: %v", nested["tag"])
	}
}

func TestRedactJSON_KeySufijos(t *testing.T) {
	in := []byte(`{"user_email":"x@y.z","stripe_token":"sk_test"}`)
	out := RedactJSON(in, nil)
	var v map[string]any
	_ = json.Unmarshal(out, &v)
	if v["user_email"] != "[REDACTED]" {
		t.Fatal("_email suffix not detected")
	}
	if v["stripe_token"] != "[REDACTED]" {
		t.Fatal("_token suffix not detected")
	}
}

func TestRedactJSON_InvalidJSON_RetornaOriginal(t *testing.T) {
	in := []byte(`not json`)
	out := RedactJSON(in, DefaultSensitiveJSONKeys)
	if string(out) != string(in) {
		t.Fatal("invalid JSON should pass through unchanged")
	}
}

func TestRedactContentTag_Determinístico(t *testing.T) {
	a := RedactContentTag("hello world")
	b := RedactContentTag("hello world")
	if a != b {
		t.Fatalf("not deterministic: %s vs %s", a, b)
	}
	if !strings.HasPrefix(a, "[REDACTED-") {
		t.Fatalf("missing prefix: %s", a)
	}
}

func TestDefaultConfig_TienePolicyParaTablasPII(t *testing.T) {
	cfg := DefaultConfig()
	required := []string{"users", "organizations", "observations", "audit_log", "auth_api_keys"}
	for _, table := range required {
		if _, ok := cfg.Tables[table]; !ok {
			t.Fatalf("DefaultConfig missing %s", table)
		}
	}
	if !cfg.Tables["auth_api_keys"].Skip {
		t.Fatal("auth_api_keys MUST be skipped")
	}
	if !cfg.Tables["auth_otp_codes"].Skip {
		t.Fatal("auth_otp_codes MUST be skipped")
	}
}
