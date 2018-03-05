package proxy

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
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

type Options struct {
	Autoindex  *bool    `json:"autoindex,omitempty" yaml:"autoindex,omitempty"`
	IndexFiles []string `json:"index_files,omitempty" yaml:"index_files,omitempty"`
}

type handler struct {
	Client s3iface.S3API
	Region string
	Bucket string
	Options
}

// NewHandler creates a new HTTP handler under the default session configuration
func NewHandler(region, bucket string, opts Options) (http.Handler, error) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return nil, err
	}

	if region != "" {
		cfg.Region = region
	}
	if cfg.Region == "" {
		return nil, errors.New("AWS region missing: you may need to set the AWS_REGION environment variable, or refer to the documentation")
	}

	if bucket == "" {
		return nil, errors.New("Bucket name is required")
	}

	return &handler{
		Client:  s3.New(cfg),
		Region:  region,
		Bucket:  bucket,
		Options: opts,
	}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path != "/" {
		path = strings.TrimPrefix(path, "/")
	}

	log := hlog.FromRequest(r)
	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.
			Str("s3_region", h.Region).
			Str("s3_bucket", h.Bucket).
			Str("s3_key", path)
	})

	if strings.HasSuffix(path, "/") {
		var foundPath string
		for _, candidate := range h.Options.IndexFiles {
			if h.hasObject(r, path+candidate) {
				foundPath = path + candidate
				break
			}
		}
		if foundPath == "" {
			if h.Options.Autoindex != nil && *h.Options.Autoindex {
				if path == "/" {
					h.serveDirectoryListing(w, r, "")
				} else {
					h.serveDirectoryListing(w, r, path)
				}
			} else {
				http.Error(w, "Could not find a valid index file. Additionally, directory listing was denied.", http.StatusForbidden)
			}
			return
		}
		path = foundPath
	}
	h.serveFile(w, r, path)
}

type dirListing struct {
	Entries     []dirEntry
	Prefixes    []string
	IsTruncated bool
}
type dirEntry struct {
	Name    string
	Size    int64
	ModTime *time.Time
}

func (h *handler) renderDirectoryListing(w http.ResponseWriter, listing dirListing) error {
	var buf bytes.Buffer
	if err := directoryListingTemplate.Execute(&buf, listing); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
	return nil
}

func (h *handler) serveDirectoryListing(w http.ResponseWriter, r *http.Request, path string) {
	log := hlog.FromRequest(r)

	// Request from S3
	obj, err := h.listObjects(r, path)
	if err != nil {
		log.Error().Err(err).Msg("generic s3 listing error")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var files []dirEntry
	for _, content := range obj.Contents {
		files = append(files, dirEntry{
			Name:    strings.TrimPrefix(aws.StringValue(content.Key), path),
			Size:    aws.Int64Value(content.Size),
			ModTime: content.LastModified,
		})
	}

	var prefixes []string
	for _, cp := range obj.CommonPrefixes {
		prefixes = append(prefixes, strings.TrimPrefix(aws.StringValue(cp.Prefix), path))
	}

	listing := dirListing{
		Entries:     files,
		Prefixes:    prefixes,
		IsTruncated: aws.BoolValue(obj.IsTruncated),
	}
	err = h.renderDirectoryListing(w, listing)
	if err != nil {
		log.Error().Err(err).Msg("directory listing render error")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *handler) serveFile(w http.ResponseWriter, r *http.Request, path string) {
	log := hlog.FromRequest(r)

	// Request from S3
	obj, err := h.getObject(path)
	if err != nil {
		if reqerr, ok := err.(awserr.RequestFailure); ok {
			log.Error().Err(reqerr).
				Int("amz_status_code", reqerr.StatusCode()).
				Str("amz_code", reqerr.Code()).
				Str("amz_request_id", reqerr.RequestID()).
				Msg("")
			http.Error(w, reqerr.Message()+" Request ID: "+reqerr.RequestID(), reqerr.StatusCode())
		} else {
			log.Error().Err(err).Msg("generic s3 download error")
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		}
		return
	}

	// Immediately handle website redirects
	if aws.StringValue(obj.WebsiteRedirectLocation) != "" {
		copyStringHeader(w, "Location", obj.WebsiteRedirectLocation)
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	if aws.StringValue(obj.ContentType) == "application/x-directory" && !strings.HasSuffix(r.RequestURI, "/") {
		dirpath := r.RequestURI + "/"
		copyStringHeader(w, "Location", &dirpath)
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	// Copy common headers from S3 to the response
	copyStringHeader(w, "Cache-Control", obj.CacheControl)
	copyStringHeader(w, "Content-Disposition", obj.ContentDisposition)
	copyStringHeader(w, "Content-Encoding", obj.ContentEncoding)
	copyStringHeader(w, "Content-Language", obj.ContentLanguage)
	copyStringHeader(w, "Content-Type", obj.ContentType)
	copyStringHeader(w, "ETag", obj.ETag)
	copyStringHeader(w, "Expires", obj.Expires)

	// Copy the Last-Modified header as long as it's not the zero value
	if t := aws.TimeValue(obj.LastModified); !t.Equal(time.Time{}) {
		s := t.UTC().Format(http.TimeFormat)
		copyStringHeader(w, "Last-Modified", &s)
	}

	// Copy meta headers
	copyStringHeader(w, "X-Amz-Version-ID", obj.VersionId)
	for k, v := range obj.Metadata {
		copyStringHeader(w, "X-Amz-Meta-"+k, &v)
	}

	// Return "204 No Content" only if a Content-Length header in fact exists AND it's zero
	if obj.ContentLength != nil {
		v := aws.Int64Value(obj.ContentLength)
		if v == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Add("Content-Length", strconv.FormatInt(v, 10))
	}

	// Prepare "206 Partial Content" if Content-Range was returned
	if aws.StringValue(obj.ContentRange) != "" {
		copyStringHeader(w, "Content-Range", obj.ContentRange)
		w.WriteHeader(http.StatusPartialContent)
	}

	if n, err := io.Copy(w, obj.Body); err != nil {
		log.Error().Err(err).Int64("bytes_written", n).Msg("")
		return
	}
}

func (h *handler) hasObject(r *http.Request, path string) bool {
	i := &s3.HeadObjectInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(path),
	}
	q := h.Client.HeadObjectRequest(i)
	q.SetContext(r.Context())
	_, err := q.Send()
	return err == nil
}

func (h *handler) getObject(path string) (*s3.GetObjectOutput, error) {
	r := &s3.GetObjectInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(path),
	}
	return h.Client.GetObjectRequest(r).Send()
}

func (h *handler) listObjects(r *http.Request, prefix string) (*s3.ListObjectsV2Output, error) {
	i := &s3.ListObjectsV2Input{
		Bucket:    aws.String(h.Bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}
	q := h.Client.ListObjectsV2Request(i)
	q.SetContext(r.Context())
	return q.Send()
}

func copyStringHeader(w http.ResponseWriter, k string, v *string) {
	if s := aws.StringValue(v); s != "" {
		w.Header().Add(k, s)
	}
}
