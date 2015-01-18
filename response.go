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
		var ewr io.WriteCloser

		// Encode response
		accept := r.Header.Get("Accept-Encoding")
		if strings.Contains(accept, "gzip") {
			ewr = gzip.NewWriter(w)
			w.Header().Set("Content-Encoding", "gzip")
		} else if strings.Contains(accept, "deflate") {
			ewr, _ = flate.NewWriter(w, flate.DefaultCompression)
			w.Header().Set("Content-Encoding", "deflate")
		}

		if ewr != nil {
			defer ewr.Close()
			rw := respWriter{w: ewr, ResponseWriter: w}
			if cn, ok := w.(http.CloseNotifier); ok {
				w = &respWriterCN{rw, cn}
			} else {
				w = &rw
			}
		}

		// Pass to the wrapped handler
		h.ServeHTTP(w, r)
	})
}

type respWriter struct {
	w io.Writer
	http.ResponseWriter
}

func (w *respWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

type respWriterCN struct {
	respWriter
	http.CloseNotifier
}
