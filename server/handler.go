package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3manager"
)

type handler struct {
	cfg aws.Config
}

// NewHandler creates a new HTTP handler under the default session configuration
func NewHandler(defaultRegion string) (http.Handler, error) {
	cfg, err := external.LoadDefaultAWSConfig()
	if cfg.Region == "" {
		if defaultRegion == "" {
			return nil, errors.New("AWS region missing: you may need to set the AWS_REGION environment variable, or refer to the documentation")
		}
		cfg.Region = defaultRegion
	}
	if err != nil {
		return nil, err
	}

	return &handler{
		cfg: cfg,
	}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cli := s3.New(h.cfg)
	cli.Region = Region(r)
	dl := s3manager.NewDownloaderWithClient(cli)

	s3req := &s3.GetObjectInput{
		Bucket: aws.String(Bucket(r)),
		Key:    aws.String(r.URL.Path),
	}
	buf := &aws.WriteAtBuffer{}
	n, err := dl.DownloadWithContext(r.Context(), buf, s3req)
	if err != nil {
		http.Error(w, fmt.Sprintf("*s3.Downloader: %s", err), http.StatusServiceUnavailable)
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
