package main

import (
	"io/ioutil"
	"net/http"
	"strings"

	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"

	"github.com/gorilla/mux"
	"github.com/ripta/ssp/server"
	"github.com/ripta/zapextra"
)

type ConfigHandler struct {
	Host       string `json:"host,omitempty" yaml:"host,omitempty"`
	PathPrefix string `json:"path_prefix,omitempty" yaml:"path_prefix,omitempty"`
	S3Bucket   string `json:"s3_bucket,omitempty" yaml:"s3_bucket,omitempty"`
	S3Prefix   string `json:"s3_prefix,omitempty" yaml:"s3_prefix,omitempty"`
}

type ConfigRoot struct {
	Handlers []*ConfigHandler `json:"handlers" yaml:"handlers"`
}

func LoadConfig(filename string) (*ConfigRoot, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	cfg := &ConfigRoot{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (ch *ConfigHandler) InjectRoute(r *mux.Router, h http.Handler, log *zap.Logger) {
	rt := r.NewRoute()
	if ch.Host != "" {
		rt = rt.Host(ch.Host)
	}
	if ch.PathPrefix != "" {
		rt = rt.PathPrefix(ch.PathPrefix)
	}
	rewritten := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req = server.SetBucket(req, ch.S3Bucket)
		if ch.S3Prefix != "" {
			p := req.URL.Path
			v := mux.Vars(req)
			if ch.PathPrefix != "" {
				p = strings.TrimPrefix(p, substituteParams(ch.PathPrefix, v))
			}
			p = substituteParams(ch.S3Prefix, v) + p
			p = strings.TrimPrefix(p, "/")
			req = server.SetObjectKey(req, p)
		}
		h.ServeHTTP(w, req)
	})
	log.Debug("Installing route", zap.Reflect("config_handler", ch))
	rt.Handler(zapextra.LoggingHandler(log, rewritten, zap.String("@tag", "ssp.access")))
}

func (cfg *ConfigRoot) InjectRoutes(r *mux.Router, h http.Handler, log *zap.Logger) {
	for _, ch := range cfg.Handlers {
		ch.InjectRoute(r, h, log)
	}
}
