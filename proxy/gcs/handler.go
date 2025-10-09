package gcs

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/ripta/ssp/proxy"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

type handler struct {
	Client  *storage.Client
	Bucket  string
	KeyFile string

	proxy.Options
}

func NewHandler(bucket, keyFile string, opts proxy.Options) (http.Handler, error) {
	c, err := newClient(context.Background(), keyFile)
	if err != nil {
		return nil, err
	}

	h := handler{
		Client:  c,
		Bucket:  bucket,
		KeyFile: keyFile,
		Options: opts,
	}
	return &h, nil
}

func newClient(ctx context.Context, keyFile string) (*storage.Client, error) {
	if keyFile == "" {
		return storage.NewClient(ctx)
	}
	return storage.NewClient(ctx, option.WithCredentialsFile(keyFile))
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	log := hlog.FromRequest(r)
	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.
			Str("gcs_bucket", h.Bucket).
			Str("gcs_key", path)
	})

	if path == "" || strings.HasSuffix(path, "/") {
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

func (h *handler) getObject(r *http.Request, path string) (*storage.ObjectHandle, *storage.ObjectAttrs, error) {
	obj := h.Client.Bucket(h.Bucket).Object(path)
	attrs, err := obj.Attrs(r.Context())
	if err != nil {
		return nil, nil, err
	}
	return obj, attrs, nil
}

func (h *handler) hasObject(r *http.Request, path string) bool {
	_, err := h.Client.Bucket(h.Bucket).Object(path).Attrs(r.Context())
	return err == nil
}

func (h *handler) listObjects(r *http.Request, prefix string) *storage.ObjectIterator {
	q := storage.Query{
		Delimiter: "/",
		Prefix:    prefix,
	}
	return h.Client.Bucket(h.Bucket).Objects(r.Context(), &q)
}

func (h *handler) serveDirectoryListing(w http.ResponseWriter, r *http.Request, path string) {
	log := hlog.FromRequest(r)

	var files []proxy.DirectoryEntry
	it := h.listObjects(r, path)
	for {
		obj, err := it.Next()
		if err != nil {
			if err == iterator.Done {
				break
			}
			log.Error().Err(err).Msg("generic GCS listing error")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		files = append(files, proxy.DirectoryEntry{
			Name:    strings.TrimPrefix(obj.Name, path),
			Size:    obj.Size,
			ModTime: &obj.Created,
		})
	}

	listing := proxy.DirectoryListing{
		Entries: files,
	}
	if err := proxy.RenderDirectoryListing(w, listing); err != nil {
		log.Error().Err(err).Msg("directory listing render error")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *handler) serveFile(w http.ResponseWriter, r *http.Request, path string) {
	log := hlog.FromRequest(r)

	// Request from GCS
	obj, attrs, err := h.getObject(r, path)
	if err != nil {
		log.Error().Err(err).Msg("")
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	if attrs.Prefix != "" {
		dirpath := r.RequestURI + "/"
		w.Header().Add("Location", dirpath)
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	// Copy common headers from S3 to the response
	copyStringHeader(w, "Cache-Control", attrs.CacheControl)
	copyStringHeader(w, "Content-Disposition", attrs.ContentDisposition)
	copyStringHeader(w, "Content-Encoding", attrs.ContentEncoding)
	copyStringHeader(w, "Content-Language", attrs.ContentLanguage)
	copyStringHeader(w, "Content-Type", attrs.ContentType)

	// Copy the Last-Modified header as long as it's not the zero value
	if t := attrs.Created; !t.Equal(time.Time{}) {
		s := t.UTC().Format(http.TimeFormat)
		copyStringHeader(w, "Last-Modified", s)
	}

	// Return "204 No Content" only if a Content-Length header in fact exists AND it's zero
	if attrs.Size == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	copyStringHeader(w, "Content-Length", strconv.FormatInt(attrs.Size, 10))

	body, err := obj.NewReader(r.Context())
	if n, err := io.Copy(w, body); err != nil {
		log.Error().Err(err).Int64("bytes_written", n).Msg("")
		return
	}
}

func copyStringHeader(w http.ResponseWriter, k, v string) {
	if v != "" {
		w.Header().Add(k, v)
	}
}
