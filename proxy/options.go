package proxy

type Options struct {
	Autoindex  *bool    `json:"autoindex,omitempty" yaml:"autoindex,omitempty"`
	IndexFiles []string `json:"index_files,omitempty" yaml:"index_files,omitempty"`
}
