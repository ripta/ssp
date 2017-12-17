package main

import (
	arg "github.com/alexflint/go-arg"
)

// Build-time variables
var (
	BuildDate    string
	BuildVersion string

	BuildEnvironment = "dev"
)

// Default variables used
const (
	AppName = "ssp"

	DefaultBucketRegion     = "us-east-1"
	DefaultBucketTeamPrefix = "team/"
	DefaultBucketUserPrefix = "user/"
	DefaultPort             = 8080
)

type options struct {
	BucketRegion     string `arg:"--bucket-region,env:SSP_BUCKET_REGION"`
	BucketTeamPrefix string `arg:"--bucket-team-prefix,env:SSP_BUCKET_TEAM_PREFIX"`
	BucketUserPrefix string `arg:"--bucket-user-prefix,env:SSP_BUCKET_USER_PREFIX"`
	Config           string `arg:"--config,env:SSP_CONFIG"`
	Environment      string `arg:"--env,env:SSP_ENV,help:Environment name 'dev' or 'prod'"`
	Port             int    `arg:"--port,env:SSP_PORT,help:Port to listen on"`
}

func (o *options) Version() string {
	return BuildVersion
}

func parseOptions() options {
	var o options

	o.BucketRegion = DefaultBucketRegion
	o.BucketTeamPrefix = DefaultBucketTeamPrefix
	o.BucketUserPrefix = DefaultBucketUserPrefix
	o.Environment = BuildEnvironment
	o.Port = DefaultPort

	arg.MustParse(&o)
	return o
}
