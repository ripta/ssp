package server

import (
	"net/http"

	"go.uber.org/zap"
)

type muxer struct {
	*http.ServeMux
	l *zap.Logger
}

func NewMux(l *zap.Logger) *muxer {
	m := &muxer{
		ServeMux: http.NewServeMux(),
		l:        l,
	}

	return m
}
