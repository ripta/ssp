package proxy

import (
	"html/template"
	"net/http"
	"time"
)

const directoryListingTemplateText = `
<!doctype html>
<html>
<body>
	<ul>
	{{- range $i, $prefix := .Prefixes }}
		<li><a href="{{ $prefix }}">{{ $prefix }}</a></li>
	{{- end }}
	{{- range $i, $entry := .Entries }}
		<li><a href="{{ $entry.Name }}">{{ $entry.Name }}</a> <em>{{ $entry.Size }} bytes</em></li>
	{{- end }}
	</ul>
</body>
</html>
`

var directoryListingTemplate = template.Must(template.New("autoindex").Parse(directoryListingTemplateText))

// DirectoryListing is a directory or prefix and all its entries.
type DirectoryListing struct {
	Entries     []DirectoryEntry
	Prefixes    []string
	IsTruncated bool
}

// DirectoryEntry is an entry within a directory.
type DirectoryEntry struct {
	Name    string
	Size    int64
	ModTime *time.Time
}

// RenderDirectoryListing renders a text/template for directory listings.
func RenderDirectoryListing(w http.ResponseWriter, listing DirectoryListing) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return directoryListingTemplate.Execute(w, listing)
}
