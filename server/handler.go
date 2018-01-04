package server

import (
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3manager"
)

type handler struct {
	cfg aws.Config
	cli *s3.S3
	dl  *s3manager.Downloader
}

// NewHandler creates a new HTTP handler under the default session configuration
func NewHandler() (http.Handler, error) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return nil, err
	}

	cli := s3.New(cfg)
	return &handler{
		cfg: cfg,
		cli: cli,
		dl:  s3manager.NewDownloaderWithClient(cli),
	}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// io.WriteString(w, "hello world!\n")
	s3req := &s3.GetObjectInput{
		Bucket: aws.String("userdir-routed-cloud"),
		Key:    aws.String(r.URL.Path),
	}
	buf := &aws.WriteAtBuffer{}
	n, err := h.dl.DownloadWithContext(r.Context(), buf, s3req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if n == 0 {
		http.Error(w, http.StatusText(http.StatusNoContent), http.StatusNoContent)
		return
	}
}
