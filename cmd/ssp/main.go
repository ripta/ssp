package main

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/ripta/ssp/server"
	"github.com/rs/zerolog"
)

var (
	reVarSubsitution = regexp.MustCompile("\\{[^}]+\\}")
)

func init() {
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

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal().Err(err).Msg("cannot listen")
	}
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
