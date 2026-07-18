package config

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

type substituteParamsTest struct {
	Name   string
	Input  string
	Params map[string]string
	Want   string
}

var substituteParamsTests = []substituteParamsTest{
	{
		Name:   "no placeholders",
		Input:  "/static",
		Params: nil,
		Want:   "/static",
	},
	{
		Name:   "single placeholder",
		Input:  "/{sub}",
		Params: map[string]string{"sub": "anything"},
		Want:   "/anything",
	},
	{
		Name:   "multiple placeholders",
		Input:  "/{a}/{b}",
		Params: map[string]string{"a": "x", "b": "y"},
		Want:   "/x/y",
	},
	{
		Name:   "missing var substitutes empty string",
		Input:  "/{missing}",
		Params: map[string]string{},
		Want:   "/",
	},
}

func TestSubstituteParams(t *testing.T) {
	for _, tt := range substituteParamsTests {
		t.Run(tt.Name, func(t *testing.T) {
			got := substituteParams(tt.Input, tt.Params)
			if got != tt.Want {
				t.Errorf("substituteParams(%q, %v) = %q, want %q", tt.Input, tt.Params, got, tt.Want)
			}
		})
	}
}

type rewriteHandlerTest struct {
	Name       string
	Handler    ConfigHandler
	Prefix     string
	Vars       map[string]string
	RequestURI string
	WantPath   string
}

var rewriteHandlerTests = []rewriteHandlerTest{
	{
		Name:       "gcs prefix substitutes captured host variable",
		Handler:    ConfigHandler{Host: "{sub}.example.com"},
		Prefix:     "/{sub}",
		Vars:       map[string]string{"sub": "anything"},
		RequestURI: "/foo.html",
		WantPath:   "/anything/foo.html",
	},
	{
		Name:       "s3 prefix substitutes captured host variable",
		Handler:    ConfigHandler{Host: "{username}.userdir.routed.cloud"},
		Prefix:     "/users/{username}",
		Vars:       map[string]string{"username": "ripta"},
		RequestURI: "/index.html",
		WantPath:   "/users/ripta/index.html",
	},
	{
		Name:       "path prefix is stripped before adding backend prefix",
		Handler:    ConfigHandler{PathPrefix: "/~{username}"},
		Prefix:     "/users/{username}",
		Vars:       map[string]string{"username": "ripta"},
		RequestURI: "/~ripta/abc.txt",
		WantPath:   "/users/ripta/abc.txt",
	},
	{
		Name:       "static prefix with no captured variables",
		Handler:    ConfigHandler{Host: "uncommon.routed.cloud"},
		Prefix:     "/hello",
		Vars:       map[string]string{},
		RequestURI: "/world.html",
		WantPath:   "/hello/world.html",
	},
}

func TestConfigHandlerRewriteHandler(t *testing.T) {
	for _, tt := range rewriteHandlerTests {
		t.Run(tt.Name, func(t *testing.T) {
			ch := tt.Handler

			var gotPath string
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
			})

			h := ch.rewriteHandler(inner, tt.Prefix)

			req := httptest.NewRequest(http.MethodGet, tt.RequestURI, nil)
			req = mux.SetURLVars(req, tt.Vars)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if gotPath != tt.WantPath {
				t.Errorf("rewritten path = %q, want %q", gotPath, tt.WantPath)
			}
			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
			}
		})
	}
}
