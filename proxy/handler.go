package server

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/rs/zerolog/hlog"
)

type handler struct {
	cfg    aws.Config
	region string
	bucket string
}

// NewHandler creates a new HTTP handler under the default session configuration
func NewHandler(region, bucket string) (http.Handler, error) {
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

	// fmt.Fprintf(os.Stderr, "set up %v in %v\n", bucket, region)
	return &handler{
		cfg:    cfg,
		region: region,
		bucket: bucket,
	}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := hlog.FromRequest(r)
	path := r.URL.Path
	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.
			Str("s3_region", h.region).
			Str("s3_bucket", h.bucket).
			Str("s3_key", path)
	})

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

	if aws.StringValue(obj.ContentRange) != "" {
		copyStringHeader(w, "Content-Range", obj.ContentRange)
		w.WriteHeader(http.StatusPartialContent)
	}

	if n, err := io.Copy(w, obj.Body); err != nil {
		log.Error().Err(err).Int64("bytes_written", n).Msg("")
		return
	}
	return
}

func (h *handler) getObject(path string) (*s3.GetObjectOutput, error) {
	r := &s3.GetObjectInput{
		Bucket: aws.String(h.bucket),
		Key:    aws.String(path),
	}
	return s3.New(h.cfg).GetObjectRequest(r).Send()
}

func copyStringHeader(w http.ResponseWriter, k string, v *string) {
	if s := aws.StringValue(v); s != "" {
		w.Header().Add(k, s)
	}
}
