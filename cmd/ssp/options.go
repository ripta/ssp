package main

import (
	"io"
	"os"

	arg "github.com/alexflint/go-arg"
	"github.com/rs/zerolog"
)

// Build-time variables
var (
	BuildDate    string
	BuildVersion string

	BuildEnvironment = "dev"
)

// Default variables used
const (
	AppName     = "ssp"
	DefaultPort = 8080
)

type options struct {
	Config      string `arg:"--config,env:SSP_CONFIG"`
	Environment string `arg:"--env,env:SSP_ENV,help:Environment name 'dev' or 'prod'"`
	Port        int    `arg:"--port,env:SSP_PORT,help:Port to listen on"`

	Log zerolog.Logger `arg:"-"`
}

func (o *options) Version() string {
	return BuildVersion
}

func parseOptions() options {
	var o options

	o.Environment = BuildEnvironment
	o.Port = DefaultPort

	arg.MustParse(&o)

	var logDevice io.Writer
	if o.Environment == "dev" {
		logDevice = zerolog.ConsoleWriter{Out: os.Stdout}
	} else {
		logDevice = os.Stdout
	}
	o.Log = zerolog.New(logDevice).With().Timestamp().Logger()
	o.Log.Level(zerolog.DebugLevel)
	return o
}
