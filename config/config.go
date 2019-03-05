package config

import (
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/ripta/ssp/proxy"
	yaml "gopkg.in/yaml.v2"
)

var (
	reVarSubsitution = regexp.MustCompile("\\{[^}]+\\}")
)

type ConfigHandler struct {
	Host       string `json:"host,omitempty" yaml:"host,omitempty"`
	Path       string `json:"path,omitempty" yaml:"path,omitempty"`
	PathPrefix string `json:"path_prefix,omitempty" yaml:"path_prefix,omitempty"`
	S3Bucket   string `json:"s3_bucket,omitempty" yaml:"s3_bucket,omitempty"`
	S3Prefix   string `json:"s3_prefix,omitempty" yaml:"s3_prefix,omitempty"`
	S3Region   string `json:"s3_region,omitempty" yaml:"s3_region,omitempty"`

	proxy.Options `yaml:",inline"`
}

type ConfigRoot struct {
	Defaults *ConfigHandler   `json:"defaults" yaml:"defaults"`
	Handlers []*ConfigHandler `json:"handlers" yaml:"handlers"`
}

func Load(filename string) (*ConfigRoot, error) {
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
	if d == nil {
		return
	}

	if ch.Autoindex == nil {
		ch.Autoindex = d.Autoindex
	}
	if ch.Host == "" {
		ch.Host = d.Host
	}
	if len(ch.IndexFiles) == 0 {
		ch.IndexFiles = d.IndexFiles
	}
	if ch.Path == "" {
		ch.Path = d.Path
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

func (ch *ConfigHandler) InjectRoute(r *mux.Router) error {
	h, err := proxy.NewHandler(ch.S3Region, ch.S3Bucket, ch.Options)
	if err != nil {
		return errors.Wrap(err, "could not initialize request handler")
	}
	if ch.S3Prefix != "" {
		h = ch.rewriteHandler(h)
	}
	ch.buildRoute(r).Handler(h)
	return nil
}

func (ch *ConfigHandler) buildRoute(r *mux.Router) *mux.Route {
	rt := r.NewRoute()
	if ch.Host != "" {
		rt = rt.Host(ch.Host)
	}
	if ch.Path != "" {
		rt = rt.Path(ch.Path)
	} else if ch.PathPrefix != "" {
		rt = rt.PathPrefix(ch.PathPrefix)
	}
	return rt.Methods("GET")
}

func (ch *ConfigHandler) rewriteHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// rewrite path
		p := req.URL.Path
		v := mux.Vars(req)
		if ch.PathPrefix != "" {
			p = strings.TrimPrefix(p, substituteParams(ch.PathPrefix, v))
		}
		p = substituteParams(ch.S3Prefix, v) + p

		// deep copy the request so we can reinject the rewritten path
		req = req.WithContext(req.Context())
		req.URL.Path = p
		h.ServeHTTP(w, req)
	})
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
