package main

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/rs/zerolog/hlog"

	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/ripta/ssp/proxy"
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
	log := newLogger(opts)

	log.Debug().Interface("options", opts).Msg("parse options")

	r := mux.NewRouter()
	r.NotFoundHandler = http.NotFoundHandler()

	if opts.Config != "" {
		cfg, err := LoadConfig(opts.Config)
		if err != nil {
			log.Fatal().Err(err).Str("config_file", opts.Config).Msg("could not load config")
		}

		// cfg.InjectRoutes(r, server.DumpRequestHandler, log)
		h, err := server.NewHandler(cfg.Defaults.S3Region)
		if err != nil {
			log.Fatal().Err(err).Msg("could not initialize request handler")
		}
		cfg.InjectRoutes(r, h, log)
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
	return chain
}

func newLogger(o options) zerolog.Logger {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if o.Environment == "prod" {
		return log.Level(zerolog.InfoLevel)
	}
	return log.Level(zerolog.DebugLevel)
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
