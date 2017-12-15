package server

import (
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
)

type handler struct {
	s *session.Session
}

func NewHandler() (*handler, error) {
	s, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	return &handler{
		s: s,
	}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "hello world!\n")
}
