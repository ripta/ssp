package server

import (
	"net/http"

	"go.uber.org/zap"
)

func MustBeAuthenticated(f func(http.ResponseWriter, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f(w, r)
	})
}

func MustLog(l *zap.Logger, f func(http.ResponseWriter, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f(w, r)
	})
}
