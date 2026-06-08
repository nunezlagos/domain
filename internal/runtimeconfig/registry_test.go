package runtimeconfig

import (
	"testing"
)

func TestDefaults(t *testing.T) {
	d := Defaults()
	if d.LogLevel != "info" {
		t.Fatalf("log_level: %s", d.LogLevel)
	}
	if d.HTTPRequestTimeoutSeconds != 30 {
		t.Fatalf("http timeout: %d", d.HTTPRequestTimeoutSeconds)
	}
	if d.OTELSampleRatio != 0.1 {
		t.Fatalf("otel sample: %v", d.OTELSampleRatio)
	}
}

func TestApplyValue_LogLevel_Valido(t *testing.T) {
	s := Defaults()
	for _, v := range []string{`"debug"`, `"info"`, `"warn"`, `"error"`} {
		if err := applyValue(s, "log_level", []byte(v)); err != nil {
			t.Fatalf("%s: %v", v, err)
		}
	}
}

func TestApplyValue_LogLevel_Invalido(t *testing.T) {
	s := Defaults()
	if err := applyValue(s, "log_level", []byte(`"random"`)); err == nil {
		t.Fatal("expected error for invalid log_level")
	}
}

func TestApplyValue_TimeoutOutOfRange(t *testing.T) {
	s := Defaults()
	if err := applyValue(s, "http_request_timeout_seconds", []byte(`9999`)); err == nil {
		t.Fatal("expected out-of-range error")
	}
	if err := applyValue(s, "http_request_timeout_seconds", []byte(`-1`)); err == nil {
		t.Fatal("expected negative error")
	}
}

func TestApplyValue_OTELRatio(t *testing.T) {
	s := Defaults()
	if err := applyValue(s, "otel_sample_ratio", []byte(`0.5`)); err != nil {
		t.Fatal(err)
	}
	if s.OTELSampleRatio != 0.5 {
		t.Fatalf("ratio: %v", s.OTELSampleRatio)
	}
	if err := applyValue(s, "otel_sample_ratio", []byte(`1.5`)); err == nil {
		t.Fatal("expected out-of-range error")
	}
}

func TestApplyValue_UnknownKey(t *testing.T) {
	s := Defaults()
	if err := applyValue(s, "random_key", []byte(`"x"`)); err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestHotReloadable_Catalog(t *testing.T) {
	required := []string{"log_level", "http_request_timeout_seconds", "metrics_enabled"}
	for _, k := range required {
		if !HotReloadable[k] {
			t.Fatalf("missing hot-reloadable key: %s", k)
		}
	}
}

func TestRegistry_Current_NilFallbacksDefaults(t *testing.T) {
	r := &Registry{}
	c := r.Current()
	if c == nil || c.LogLevel != "info" {
		t.Fatal("Current should fallback to Defaults")
	}
}
