package httpzip

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// NewResponseHandler returns handler which transparently compresses response
// written by passed handler h. The compression algorithm is being chosen
// accordingly to the value of Accept-Encoding header: both gzip and deflate
// are supported, with gzip taking precedence if both are present.
//
// The returned handler preserves http.CloseNotifier implementation of h, if any.
func NewResponseHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var compressor flushWriter

		// Encode response
		accept := r.Header.Get("Accept-Encoding")
		if strings.Contains(accept, "gzip") {
			compressor = gzip.NewWriter(w)
			w.Header().Set("Content-Encoding", "gzip")
		} else if strings.Contains(accept, "deflate") {
			compressor, _ = flate.NewWriter(w, flate.DefaultCompression)
			w.Header().Set("Content-Encoding", "deflate")
		}

		if compressor != nil {
			defer compressor.Close()
			rw := respWriter{cw: compressor, ResponseWriter: w}
			if v, ok := w.(http.CloseNotifier); ok {
				rw.CloseNotifier = v
			}
			if v, ok := w.(http.Hijacker); ok {
				rw.Hijacker = v
			}
			w = &rw
		}

		// Pass to the wrapped handler
		h.ServeHTTP(w, r)
	})
}

type flushWriter interface {
	io.WriteCloser
	Flush() error
}

type respWriter struct {
	http.ResponseWriter

	// Compressing writer. Should be passing compressed data to ResponseWriter.
	cw flushWriter

	// Interfaces form http package implemented by standard ResponseWriter.
	// May be nil if wrapped ResponseWriter doesn't implement them.
	http.CloseNotifier
	http.Hijacker
}

func (w *respWriter) Write(p []byte) (int, error) {
	return w.cw.Write(p)
}

func (w *respWriter) Flush() {
	_ = w.cw.Flush()
	if f, ok := w.ResponseWriter.(http.Flusher); ok && f != nil {
		f.Flush()
	}
}
