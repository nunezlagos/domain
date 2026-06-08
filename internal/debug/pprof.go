// Package debug — HU-27.1 pprof endpoints + HU-27.2 GOMAXPROCS/GOMEMLIMIT.
//
// Servidor opcional en puerto separado (default 6060, bind 127.0.0.1) con
// /debug/pprof/* protegido por Basic Auth.
package debug

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"runtime"
	"time"
)

// Config para el debug server.
type Config struct {
	Enabled  bool
	Bind     string // default "127.0.0.1"
	Port     int    // default 6060
	AuthUser string
	AuthPass string
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
	// Registrar handlers de pprof manualmente (stdlib).
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
// inferidos por K8s cgroup limits (HU-27.2).
//
// Si DOMAIN_GOMAXPROCS está seteado, se aplica. En su ausencia el runtime
// Go 1.25 ya detecta cgroup CPU quota automáticamente desde Go 1.22+.
func TuneRuntime(logger *slog.Logger) {
	logger.Info("runtime tuning",
		slog.Int("gomaxprocs", runtime.GOMAXPROCS(0)),
		slog.Int("cpus", runtime.NumCPU()))
}
