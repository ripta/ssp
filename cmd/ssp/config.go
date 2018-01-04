package main

import (
	"io/ioutil"
	"net/http"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/gorilla/mux"
	"github.com/ripta/ssp/server"
	"github.com/rs/zerolog"
)

type ConfigHandler struct {
	Autoindex  *bool    `json:"autoindex,omitempty" yaml:"autoindex,omitempty"`
	Host       string   `json:"host,omitempty" yaml:"host,omitempty"`
	IndexFiles []string `json:"index_files,omitempty" yaml:"index_files,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty" yaml:"path_prefix,omitempty"`
	S3Bucket   string   `json:"s3_bucket,omitempty" yaml:"s3_bucket,omitempty"`
	S3Prefix   string   `json:"s3_prefix,omitempty" yaml:"s3_prefix,omitempty"`
	S3Region   string   `json:"s3_region,omitempty" yaml:"s3_region,omitempty"`
}

type ConfigRoot struct {
	Defaults *ConfigHandler   `json:"defaults" yaml:"defaults"`
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

	for _, ch := range cfg.Handlers {
		ch.setDefaults(cfg.Defaults)
	}
	return cfg, nil
}

func (ch *ConfigHandler) setDefaults(d *ConfigHandler) {
	if ch.Autoindex == nil {
		ch.Autoindex = d.Autoindex
	}
	if ch.Host == "" {
		ch.Host = d.Host
	}
	if len(ch.IndexFiles) == 0 {
		ch.IndexFiles = d.IndexFiles
	}
	if ch.PathPrefix == "" {
		ch.PathPrefix = d.PathPrefix
	}
	if ch.S3Bucket == "" {
		ch.S3Bucket = d.S3Bucket
	}
	if ch.S3Prefix == "" {
		ch.S3Prefix = d.S3Prefix
	}
	if ch.S3Region == "" {
		ch.S3Region = d.S3Region
	}
}

func (ch *ConfigHandler) InjectRoute(r *mux.Router, h http.Handler, log zerolog.Logger) {
	rt := r.NewRoute()
	if ch.Host != "" {
		rt = rt.Host(ch.Host)
	}
	if ch.PathPrefix != "" {
		rt = rt.PathPrefix(ch.PathPrefix)
	}
	rewritten := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req = server.SetRegionBucket(req, ch.S3Region, ch.S3Bucket)
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
	log.Debug().Interface("config_handler", ch).Msg("installing route")
	rt.Handler(rewritten)
}

func (cfg *ConfigRoot) InjectRoutes(r *mux.Router, h http.Handler, log zerolog.Logger) {
	for _, ch := range cfg.Handlers {
		ch.InjectRoute(r, h, log)
	}
}
