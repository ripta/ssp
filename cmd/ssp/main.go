package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/gorilla/mux"
	"github.com/ripta/ssp/server"
	"github.com/ripta/zapextra"
)

var (
	reVarSubsitution = regexp.MustCompile("\\{[^}]+\\}")
)

func main() {
	opts := parseOptions()
	log, err := newLogger(opts)
	if err != nil {
		panic(err)
	}

	log.Debug("Parsed options", zap.Reflect("options", opts))

	r := mux.NewRouter()
	r.NotFoundHandler = zapextra.LoggingHandler(log, http.NotFoundHandler())

	if opts.Config != "" {
		cfg, err := LoadConfig(opts.Config)
		if err != nil {
			log.Fatal("Could not load config", zap.String("config_file", opts.Config), zap.Error(err))
		}

		// cfg.InjectRoutes(r, server.DumpRequestHandler, log)
		h, err := server.NewHandler(log, cfg.Defaults.S3Region)
		if err != nil {
			log.Fatal("Could not initialize request handler", zap.Error(err))
		}
		cfg.InjectRoutes(r, h, log)
	}

	port := strconv.Itoa(opts.Port)
	log.Info(fmt.Sprintf("Ready to serve requests on port %s", port))

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(fmt.Sprintf("%v", err))
	}
}

func newLogger(o options) (*zap.Logger, error) {
	c := zap.NewDevelopmentConfig()
	if o.Environment == "prod" {
		c = zap.NewProductionConfig()
	}
	c.Level.SetLevel(zap.DebugLevel)

	c.EncoderConfig.MessageKey = "message"
	c.EncoderConfig.CallerKey = ""
	c.EncoderConfig.LevelKey = "level"
	c.EncoderConfig.TimeKey = "@timestamp"
	c.EncoderConfig.StacktraceKey = "@trace"

	c.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return c.Build()
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
