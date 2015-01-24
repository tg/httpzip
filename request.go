package httpzip

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
)

// NewRequestHandler return handler, which transparently decodes http requests
// which are using either gzip or deflate algorithm. Request should have
// Content-Encoding header set to the appropriate value. If content encoding is
// recognised, request body will be transparently uncompressed in the passed
// handler h and Content-Encoding header removed. No decoding errors are
// handled by the wrapper and they're all available through the regular request
// body read call.
//
// If content encoding is outside of the supported types, the request will be
// passed unaltered.
func NewRequestHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// New reader handling uncompression
		var nr io.ReadCloser

		switch r.Header.Get("Content-Encoding") {
		case "gzip":
			if r, err := gzip.NewReader(r.Body); err == nil {
				nr = r
			} else {
				nr = ioutil.NopCloser(&errReader{err})
			}
		case "deflate":
			nr = flate.NewReader(r.Body)
		}

		if nr != nil {
			r.Body = nr
			r.Header.Del("Content-Encoding")
		}

		// Pass to the wrapped handler
		h.ServeHTTP(w, r)
	})
}

// errReader reports error on every Read
type errReader struct {
	err error
}

func (r *errReader) Read(p []byte) (int, error) {
	return 0, r.err
}
