package main

import (
	"encoding/json"
	"io/ioutil"

	"github.com/gorilla/mux"
)

type ConfigHandler struct {
	Host       string `json:"host,omitempty" yaml:"host,omitempty"`
	PathPrefix string `json:"path_prefix,omitempty" yaml:"path_prefix,omitempty"`
	S3Bucket   string `json:"s3_bucket,omitempty" yaml:"s3_bucket,omitempty"`
	S3Prefix   string `json:"s3_prefix,omitempty" yaml:"s3_prefix,omitempty"`
}

type ConfigRoot struct {
	Handlers []ConfigHandler `json:"handlers" yaml:"handlers"`
}

func LoadConfig(filename string) (*ConfigRoot, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	cfg := &ConfigRoot{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (cfg *ConfigRoot) HTTPRouter() *mux.Router {
	r := mux.NewRouter()
	return r
}
