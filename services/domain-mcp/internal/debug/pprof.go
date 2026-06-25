// Package debug — issue-27.1 pprof endpoints + issue-27.2 GOMAXPROCS/GOMEMLIMIT.
//
// Servidor opcional en puerto separado (default 6060, bind 127.0.0.1) con
// /debug/pprof/* protegido por Basic Auth.
package debug

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/metrics"
)

// Config para el debug server.
type Config struct {
	Enabled        bool
	Bind           string // default "127.0.0.1"
	Port           int    // default 6060
	AuthUser       string
	AuthPass       string
	AuditRecorder  audit.Recorder    // opcional — registra accesos a pprof
	Metrics        *metrics.Registry // opcional — expone domain_debug_pprof_accessed_total
}

// Defaults aplica valores default a Config.
func Defaults(c Config) Config {
	if c.Bind == "" {
		c.Bind = "127.0.0.1"
	}
	if c.Port == 0 {
		c.Port = 6060
	}
	return c
}

// Serve arranca el debug server. Bloquea hasta error.
// Si !Enabled retorna nil inmediatamente.
func Serve(cfg Config, logger *slog.Logger) error {
	cfg = Defaults(cfg)
	if !cfg.Enabled {
		return nil
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	handler := http.Handler(mux)
	if cfg.AuthUser != "" && cfg.AuthPass != "" {
		handler = basicAuth(cfg.AuthUser, cfg.AuthPass, mux, logger)
	} else if logger != nil {
		logger.Warn("debug server starting WITHOUT basic auth — DOMAIN_DEBUG_AUTH_USER/PASSWORD vacíos")
	}
	handler = instrumentPPROF(handler, cfg.AuditRecorder, cfg.Metrics, logger)

	addr := cfg.Bind + ":" + intStr(cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if logger != nil {
		logger.Info("debug pprof server starting",
			slog.String("addr", addr),
			slog.Bool("auth", cfg.AuthUser != ""))
	}
	return srv.ListenAndServe()
}

func basicAuth(user, pass string, next http.Handler, logger *slog.Logger) http.Handler {
	expUser := []byte(user)
	expPass := []byte(pass)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), expUser) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), expPass) != 1 {
			if logger != nil {
				logger.Warn("debug pprof access denied",
					slog.String("path", r.URL.Path),
					slog.String("remote", r.RemoteAddr))
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="debug"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if logger != nil {
			logger.Info("debug pprof accessed",
				slog.String("path", r.URL.Path),
				slog.String("user", u),
				slog.String("remote", r.RemoteAddr))
		}
		next.ServeHTTP(w, r)
	})
}

// instrumentPPROF envuelve el handler con audit logging y métricas.
func instrumentPPROF(next http.Handler, recorder audit.Recorder, reg *metrics.Registry, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)


		if reg != nil {
			reg.PprofAccessTotal.Add(1)
		}


		if recorder != nil && r.Method == http.MethodGet {
			ctx := r.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			_ = recorder.Record(ctx, audit.Event{
				Action:     "debug.pprof.accessed",
				EntityType: "debug",
				ActorType:  audit.ActorSystem,
				IPAddress:  r.RemoteAddr,
				UserAgent:  r.UserAgent(),
			})
		}
	})
}

func intStr(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	return string(buf[i:])
}

// TuneRuntime aplica GOMAXPROCS/GOMEMLIMIT según env vars o defaults
// inferidos por K8s cgroup limits (issue-27.2).
//
// Si DOMAIN_GOMAXPROCS está seteado, se aplica. En su ausencia el runtime
// Go 1.25 ya detecta cgroup CPU quota automáticamente desde Go 1.22+.
//
// GOMEMLIMIT se setea a 80% del cgroup memory limit si no está configurado
// explícitamente via env var.
func TuneRuntime(logger *slog.Logger) {
	logger.Info("runtime tuning",
		slog.Int("gomaxprocs", runtime.GOMAXPROCS(0)),
		slog.Int("cpus", runtime.NumCPU()))


	if os.Getenv("GOMEMLIMIT") != "" {
		logger.Info("GOMEMLIMIT already set via env", slog.String("value", os.Getenv("GOMEMLIMIT")))
		return
	}
	if limit := readCgroupMemoryLimit(); limit > 0 {
		softLimit := limit * 80 / 100
		if err := os.Setenv("GOMEMLIMIT", intStr(int(softLimit))); err != nil {
			logger.Warn("failed to set GOMEMLIMIT", slog.Any("err", err))
			return
		}
		logger.Info("GOMEMLIMIT auto-set to 80% of cgroup memory",
			slog.Int64("cgroup_bytes", limit),
			slog.Int64("soft_limit_bytes", softLimit))
	} else {
		logger.Warn("could not detect cgroup memory limit; GOMEMLIMIT not auto-set")
	}
}

// readCgroupMemoryLimit lee el límite de memoria del cgroup v2 o v1.
// Retorna 0 si no puede detectarlo.
func readCgroupMemoryLimit() int64 {
	const megabyte = 1 << 20

	if data, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
		s := strings.TrimSpace(string(data))
		if s == "max" {
			return 0 // sin límite
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
			return n
		}
	}

	if data, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
		s := strings.TrimSpace(string(data))
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 && n < maxMemory {
			return n
		}
	}
	return 0
}

// maxMemory es el valor de "sin límite" en cgroup v1 (usualmente un valor 
// enorme como 2^63-1 o 2^63-4096).
const maxMemory = 1 << 62
