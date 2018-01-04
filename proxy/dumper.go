package server

import (
	"fmt"
	"io"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
)

var DumpRequestHandler = http.HandlerFunc(dumpRequest)

func dumpRequest(w http.ResponseWriter, r *http.Request) {
	d := spew.NewDefaultConfig()

	io.WriteString(w, fmt.Sprintf("Mapped to s3://%s/%s\n", Bucket(r), ObjectKey(r)))
	io.WriteString(w, fmt.Sprintf("URL: %q\n", r.URL.Path))
	io.WriteString(w, fmt.Sprintf("Host: %q\n", r.Host))
	io.WriteString(w, fmt.Sprintf("RequestURI: %q\n", r.RequestURI))
	io.WriteString(w, fmt.Sprintf("User-Agent: %q\n", r.UserAgent()))

	q := d.Sdump(r.URL.Query())
	io.WriteString(w, fmt.Sprintf("Query: %s", q))

	params := mux.Vars(r)
	io.WriteString(w, "Params: "+d.Sdump(params))
	io.WriteString(w, d.Sdump(r))
}
