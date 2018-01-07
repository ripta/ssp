package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/lox/httpcache"
	"github.com/rs/zerolog/hlog"

	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/rs/zerolog"
)

var (
	reVarSubsitution = regexp.MustCompile("\\{[^}]+\\}")
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
	log.Info().Msgf("ssp v%s", opts.Version())

	r := mux.NewRouter()
	r.Path("/healthz").HandlerFunc(healthzHandler)
	r.NotFoundHandler = http.NotFoundHandler()

	if opts.Config != "" {
		cfg, err := LoadConfig(opts.Config)
		if err != nil {
			log.Fatal().Err(err).Str("config_file", opts.Config).Msg("could not load config")
		}

		// cfg.InjectRoutes(r, server.DumpRequestHandler, log)
		for _, ch := range cfg.Handlers {
			rl := log.With().Interface("route", ch).Logger()
			if err := ch.InjectRoute(r); err != nil {
				log.Fatal().Err(err).Msg("route could not be installed")
			} else {
				rl.Debug().Msg("route installed")
			}
		}
	}

	port := strconv.Itoa(opts.Port)
	log.Info().Msg(fmt.Sprintf("Ready to serve requests on port %s", port))

	chain := newHandlerChain(log, opts)
	if err := http.ListenAndServe(":"+port, chain.Then(r)); err != nil {
		log.Fatal().Err(err).Msg("cannot listen")
	}
}

func accessLogger(r *http.Request, status, size int, dur time.Duration) {
	hlog.FromRequest(r).Info().
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

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func newHandlerChain(log zerolog.Logger, opts options) alice.Chain {
	// Inject the logging device as early as possible in the chain
	chain := alice.New(hlog.NewHandler(log), hlog.AccessHandler(accessLogger))
	// Add all handlers that inject further information for the access logger
	chain = chain.Append(
		hlog.MethodHandler("method"),
		hlog.RefererHandler("referer"),
		hlog.RemoteAddrHandler("remote_addr"),
		hlog.RequestIDHandler("request_id", "X-Request-ID"),
		hlog.URLHandler("path"),
		hlog.UserAgentHandler("user_agent"),
	)
	// Enforce a timeout on anything further in the chain
	chain = chain.Append(timeoutHandler(10*time.Second, "timed out"))
	// In-memory caching is optional
	if opts.UseCache {
		log.Info().Msg("Enabled in-memory cache")
		chain = chain.Append(cachingHandlerGenerator(httpcache.NewMemoryCache()))
	}
	return chain
}

func substituteParams(s string, params map[string]string) string {
	return reVarSubsitution.ReplaceAllStringFunc(s, func(in string) string {
		k := in[1 : len(in)-1]
		v, ok := params[k]
		if ok {
			return v
		}
		return ""
	})
}

func timeoutHandler(dt time.Duration, msg string) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.TimeoutHandler(h, dt, msg)
	}
}
