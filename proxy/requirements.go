package server

import (
	"net/http"
)

func MustBeAuthenticated(f func(http.ResponseWriter, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f(w, r)
	})
}
