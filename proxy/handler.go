package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3manager"
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

	cli := s3.New(h.cfg)
	dl := s3manager.NewDownloaderWithClient(cli)

	path := strings.TrimPrefix(r.URL.Path, "/")
	s3req := &s3.GetObjectInput{
		Bucket: aws.String(h.bucket),
		Key:    aws.String(path),
	}
	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.
			Str("s3_region", h.region).
			Str("s3_bucket", h.bucket).
			Str("s3_key", path)
	})

	buf := &aws.WriteAtBuffer{}
	n, err := dl.DownloadWithContext(r.Context(), buf, s3req)
	if err != nil {
		if reqerr, ok := err.(awserr.RequestFailure); ok {
			log.Error().Err(err).
				Int("amz_status_code", reqerr.StatusCode()).
				Str("amz_code", reqerr.Code()).
				Str("amz_request_id", reqerr.RequestID()).
				Msg(reqerr.Message())
			http.Error(w, reqerr.Message()+" Request ID: "+reqerr.RequestID(), reqerr.StatusCode())
		} else {
			log.Error().Err(err).Msg("generic s3 download error")
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		}
		return
	}
	if n == 0 {
		http.Error(w, http.StatusText(http.StatusNoContent), http.StatusNoContent)
		return
	}
	n2, err := w.Write(buf.Bytes())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if n2 != int(n) {
		http.Error(w, fmt.Sprintf("Expected to write %d bytes, but wrote %d bytes", n, n2), http.StatusInternalServerError)
		return
	}
	return
}
