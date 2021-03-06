package main

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/lox/httpcache"
	"github.com/ripta/ssp/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"gopkg.in/yaml.v2"
)

func init() {
	zerolog.DurationFieldUnit = time.Millisecond
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.000000Z0700"
	zerolog.TimestampFieldName = "@timestamp"
}

func main() {
	opts := parseOptions()
	log := opts.Log
	log.Debug().Interface("options", opts).Msg("parsed options")
	if version := opts.Version(); version != "" {
		log.Info().Msgf("ssp %s (built %s)", version, BuildDate)
	}

	if opts.Config == "" {
		log.Fatal().Msg("config must not be empty")
	}

	cfg, err := config.Load(opts.Config)
	if err != nil {
		log.Fatal().Err(err).Str("config_file", opts.Config).Msg("could not load config")
	}

	r := mux.NewRouter()
	r.Path("/healthz").HandlerFunc(healthzHandler)
	r.NotFoundHandler = unknownHostHandler(cfg.Debug)

	if cfg.Debug {
		r.Path("/modulesz").HandlerFunc(debugModuleHandler)
	}

	for _, ch := range cfg.Handlers {
		rl := log.With().Interface("route", ch).Logger()
		if err := ch.InjectRoute(r); err != nil {
			log.Fatal().Err(err).Msg("route could not be installed")
		} else {
			rl.Debug().Msg("route installed")
		}
	}

	port := strconv.Itoa(opts.Port)
	log.Info().Msg(fmt.Sprintf("Ready to serve requests on port %s", port))

	chain := newHandlerChain(log, cfg)
	if err := http.ListenAndServe(":"+port, chain.Then(r)); err != nil {
		log.Fatal().Err(err).Msg("cannot listen")
	}
}

func accessLogger(r *http.Request, status, size int, dur time.Duration) {
	hlog.FromRequest(r).Info().
		Str("scheme", r.URL.Scheme).
		Str("host", r.Host).
		Int("status", status).
		Int("size", size).
		Dur("duration_ms", dur).
		Msg("request")
}

func cachingHandlerGenerator(c httpcache.Cache) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return httpcache.NewHandler(c, h)
	}
}

func debugModuleHandler(w http.ResponseWriter, r *http.Request) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		http.Error(w, "build info not available", http.StatusNoContent)
		return
	}

	p, err := yaml.Marshal(bi)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(p)
	return
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func newHandlerChain(log zerolog.Logger, cfg *config.ConfigRoot) alice.Chain {
	// Inject the logging device as early as possible in the chain
	chain := alice.New(hlog.NewHandler(log))

	// Add all handlers that inject further information for the access logger
	chain = chain.Append(
		hlog.MethodHandler("method"),
		hlog.RefererHandler("referer"),
		hlog.RemoteAddrHandler("remote_addr"),
		hlog.RequestIDHandler("request_id", "X-Request-ID"),
		hlog.URLHandler("path"),
		hlog.UserAgentHandler("user_agent"),
	)

	if h := cfg.Proxy.TrustForwardedHeaders; h != nil && *h {
		chain = chain.Append(proxyHeaderRewriteHandler)
	}

	chain = chain.Append(hlog.AccessHandler(accessLogger))

	// Enforce a timeout on anything further in the chain
	d := 10 * time.Second
	if t := cfg.Proxy.TimeoutDuration; t != nil {
		d = *t
	}
	chain = chain.Append(timeoutHandler(d, "timed out"))

	// In-memory caching is optional
	if e := cfg.Cache.Enable; e != nil && *e {
		log.Info().Msg("Enabled in-memory cache")
		chain = chain.Append(cachingHandlerGenerator(httpcache.NewMemoryCache()))
	}
	return chain
}

// proxyHeaderRewriteHandler is a partial reimplementation of gorilla toolkit's
// handlers.ProxyHeaders that _only_ looks at X-Forwarded-Host. Rewriting the other
// headers seem to break matching in gorilla mux, even with the route.Schemes(...)
// set to ["http", "https"].
func proxyHeaderRewriteHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get(http.CanonicalHeaderKey("X-Forwarded-Host")); v != "" {
			r.Host = v
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func timeoutHandler(dt time.Duration, msg string) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.TimeoutHandler(h, dt, msg)
	}
}

func unknownHostHandler(debug bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !debug {
			http.Error(w, fmt.Sprintf("404 unknown route handler for host %q", r.Host), http.StatusNotFound)
			return
		}

		p, _ := yaml.Marshal(r.Header)
		msg := fmt.Sprintf("404 unknown route handler for host %q:\n---\n%s", r.Host, string(p))
		http.Error(w, msg, http.StatusNotFound)
	}
}
