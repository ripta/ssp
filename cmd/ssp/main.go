package main

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/gorilla/mux"
	"github.com/ripta/ssp/server"
)

var (
	reVarSubsitution = regexp.MustCompile("\\{[^}]+\\}")
)

func main() {
	opts := parseOptions()
	log, err := newLogger(opts)
	if err != nil {
		panic(err)
	}

	log.Debug("Parsed options", zap.Reflect("options", opts))

	r := mux.NewRouter()
	r.NotFoundHandler = LoggingHandler(log, http.NotFoundHandler())
	// r.Host("userdir.routed.cloud").PathPrefix("/~{username}").
	// 	Handler(LoggingHandler(log, stripPath("/~{username}", prependPath("/users/{username}", server.DumpRequestHandler))))
	// r.Host("{username}.userdir.routed.cloud").
	// 	Handler(LoggingHandler(log, prependPath("/users/{username}", server.DumpRequestHandler)))

	if opts.Config != "" {
		cfg, err := LoadConfig(opts.Config)
		if err != nil {
			log.Fatal("Could not load config", zap.String("config_file", opts.Config), zap.Error(err))
		}
		cfg.InjectRoutes(r, server.DumpRequestHandler, log)
	}

	port := strconv.Itoa(opts.Port)
	log.Info(fmt.Sprintf("Ready to serve requests on port %s", port))

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(fmt.Sprintf("%v", err))
	}
}

func newLogger(o options) (*zap.Logger, error) {
	c := zap.NewDevelopmentConfig()
	if o.Environment == "prod" {
		c = zap.NewProductionConfig()
	}
	c.Level.SetLevel(zap.DebugLevel)

	c.EncoderConfig.MessageKey = "message"
	c.EncoderConfig.CallerKey = ""
	c.EncoderConfig.LevelKey = "level"
	c.EncoderConfig.TimeKey = "@timestamp"
	c.EncoderConfig.StacktraceKey = "@trace"

	c.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return c.Build()
}

func prependPath(prefix string, h http.Handler) http.Handler {
	if prefix == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = substituteParams(prefix, mux.Vars(r)) + r.URL.Path
		h.ServeHTTP(w, r)
	})
}

func stripPath(prefix string, h http.Handler) http.Handler {
	if prefix == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calcPrefix := substituteParams(prefix, mux.Vars(r))
		r.URL.Path = strings.TrimPrefix(r.URL.Path, calcPrefix)
		h.ServeHTTP(w, r)
		// if p := strings.TrimPrefix(r.URL.Path, calcPrefix); len(p) < len(r.URL.Path) {
		// 	r2 := new(http.Request)
		// 	*r2 = *r
		// 	r2.URL = new(url.URL)
		// 	*r2.URL = *r.URL
		// 	r2.URL.Path = p
		// 	h.ServeHTTP(w, r2)
		// } else {
		// 	http.NotFound(w, r)
		// }
	})
}

func stripPathComponent(c int, h http.Handler) http.Handler {
	if c < 1 {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		for c > 0 {
			p = strings.TrimPrefix(p, "/")
			i := strings.Index(p, "/")
			if i <= 0 {
				break
			}
			r.URL.Path = p[i:]
			h.ServeHTTP(w, r)
			c--
		}
		if c == 1 {
			r.URL.Path = "/"
			h.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
	})
}

func substituteParams(s string, params map[string]string) string {
	return reVarSubsitution.ReplaceAllStringFunc(s, func(in string) string {
		k := in[1 : len(in)-1]
		v, ok := params[k]
		if ok {
			return v
		}
		return ""
	})
}

// Ensure responseSizer is always a ResponseWriter at compile time
var _ http.ResponseWriter = &responseSizer{}

type responseSizer struct {
	w    http.ResponseWriter
	code int
	size uint64
}

func (s *responseSizer) Header() http.Header {
	return s.w.Header()
}

func (s *responseSizer) Write(b []byte) (int, error) {
	n, err := s.w.Write(b)
	atomic.AddUint64(&s.size, uint64(n))
	return n, err
}

func (s *responseSizer) WriteHeader(code int) {
	s.w.WriteHeader(code)
	s.code = code
}

func LoggingHandler(l *zap.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := &responseSizer{w: w}
		ts := time.Now()
		h.ServeHTTP(s, r)
		elapsed := time.Since(ts)
		l.Info(
			"Request",
			zap.String("@tag", "ssp.access"),
			zap.String("host", r.Host),
			zap.String("remote_addr", getHttpHostname(r.RemoteAddr)),
			zap.String("username", "-"),
			zap.String("method", r.Method),
			zap.String("path", r.RequestURI),
			zap.Int("status", s.code),
			zap.Uint64("size", s.size),
			zap.Duration("duration_human", elapsed),
			zap.Int64("duration_ns", elapsed.Nanoseconds()),
		)
	})
}

func getHttpHostname(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
