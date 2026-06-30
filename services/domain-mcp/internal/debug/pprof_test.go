package debug

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"nunezlagos/domain/internal/audit"
)

func TestBasicAuth_DeniesWithoutCreds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/x", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := basicAuth("u", "p", mux, nil)
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
	if resp.Header.Get("WWW-Authenticate") == "" {
		t.Fatal("missing WWW-Authenticate header")
	}
}

func TestBasicAuth_AcceptsValidCreds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/x", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := basicAuth("u", "p", mux, nil)
	srv := httptest.NewServer(h)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/x", nil)
	req.SetBasicAuth("u", "p")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestBasicAuth_RejectsWrongCreds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/x", func(w http.ResponseWriter, _ *http.Request) {})
	h := basicAuth("u", "p", mux, nil)
	srv := httptest.NewServer(h)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/x", nil)
	req.SetBasicAuth("u", "wrong")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestDefaults_AppliesBindAndPort(t *testing.T) {
	c := Defaults(Config{})
	if c.Bind != "127.0.0.1" {
		t.Fatalf("bind: %s", c.Bind)
	}
	if c.Port != 6060 {
		t.Fatalf("port: %d", c.Port)
	}
}

func TestIntStr(t *testing.T) {
	cases := map[int]string{0: "0", 1: "1", 42: "42", 6060: "6060"}
	for in, want := range cases {
		if got := intStr(in); got != want {
			t.Fatalf("intStr(%d) = %s, want %s", in, got, want)
		}
	}
}

func TestInstrumentPPROF_CallsNextHandler(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := instrumentPPROF(next, nil, nil, nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/debug/pprof/heap")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if !called {
		t.Fatal("next handler was not called")
	}
}

func TestReadCgroupMemoryLimit_ReturnsZeroWhenNoCgroup(t *testing.T) {
	limit := readCgroupMemoryLimit()

	if limit < 0 {
		t.Fatalf("expected non-negative, got %d", limit)
	}
}

func TestTuneRuntime_DoesNotPanic(t *testing.T) {

	TuneRuntime(slog.Default())
}

func TestInstrumentPPROF_NopRecorderDoesNotPanic(t *testing.T) {
	rec := &audit.NopRecorder{}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := instrumentPPROF(next, rec, nil, nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/debug/pprof/heap")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

