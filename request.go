package httpzip

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
)

// RequestHandler wraps http.Handler and provide decompression
type RequestHandler struct {
	http.Handler
}

// NewRequestHandler wraps handler with RequestHandler
func NewRequestHandler(h http.Handler) http.Handler {
	return &RequestHandler{h}
}

// ServeHTTP serves request and provides automatic decompression of gzip or
// deflate. If Content-Encoding header contains recognised algorithm, the
// request body is wrapped with uncompressor and header deleted.
func (h *RequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var br *errReader

	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		br = newErrReader(gzip.NewReader(r.Body))
	case "deflate":
		br = newErrReader(flate.NewReader(r.Body), nil)
	}

	if br != nil {
		r.Body = br
		r.Header.Del("Content-Encoding")
	}

	// Pass to the wrapped handler
	h.Handler.ServeHTTP(w, r)
}

type errReader struct {
	io.ReadCloser
	err error
}

func newErrReader(r io.ReadCloser, err error) *errReader {
	return &errReader{r, err}
}

func (r *errReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.ReadCloser.Read(p)
}
