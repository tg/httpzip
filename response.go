package httpzip

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"strings"
)

type encMethod string

const (
	// Encoding methods implemented by this library.
	// Names should match expected http header values.
	encGzip    = encMethod("gzip")
	encDeflate = encMethod("deflate")

	// Size of buffer to store initial uncompressed data.
	// Should be at least 512 to comply with detectContentType requirment.
	initBufferSize = 512
)

// NewResponseHandler returns handler which transparently compresses response
// written by passed handler h. The compression algorithm is being chosen
// accordingly to the value of Accept-Encoding header: both gzip and deflate
// are supported, with gzip taking precedence if both are present.
//
// The returned handler preserves http.CloseNotifier implementation of h, if any.
func NewResponseHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var enc encMethod

		// Encode response
		accept := r.Header.Get("Accept-Encoding")
		if strings.Contains(accept, string(encGzip)) {
			enc = encGzip
		} else if strings.Contains(accept, string(encDeflate)) {
			enc = encDeflate
		}

		if enc != "" {
			rw := newResponseWriter(w, enc)
			defer rw.Close()
			w = rw
		}

		// Pass to the wrapped handler
		w.Header().Add("Vary", "Accept-Encoding")
		h.ServeHTTP(w, r)
	})
}

// responseWriter is a ResponseWriter wrapper that will be provided to user
type responseWriter struct {
	http.ResponseWriter // original response writer

	method encMethod

	buf []byte
	cw  compressor
	err error

	// Interfaces form http package implemented by standard ResponseWriter.
	// May be nil if wrapped ResponseWriter doesn't implement them.
	http.CloseNotifier
	http.Hijacker
}

func newResponseWriter(rw http.ResponseWriter, method encMethod) *responseWriter {
	r := &responseWriter{
		ResponseWriter: rw,
		method:         method,
		buf:            make([]byte, 0, initBufferSize),
		cw:             nil,
		err:            nil,
	}

	if v, ok := rw.(http.CloseNotifier); ok {
		r.CloseNotifier = v
	}
	if v, ok := rw.(http.Hijacker); ok {
		r.Hijacker = v
	}
	return r
}

func (w *responseWriter) Write(p []byte) (nn int, err error) {
	if w.err != nil {
		return 0, w.err
	}

	if w.buf != nil {
		n := copy(w.buf[len(w.buf):cap(w.buf)], p)
		w.buf = w.buf[:len(w.buf)+n]
		p = p[n:]
		if len(w.buf) == cap(w.buf) {
			w.err = w.initCompressor(true)
			if w.err != nil {
				return 0, w.err
			}
		}
		nn = n
	}

	if len(p) > 0 && w.err == nil {
		var n int
		n, err = w.cw.Write(p)
		nn += n
	}

	return
}

// WriteHeader is called before any Write and usually in case of an error.
// As we don't know how much data is going to be written (if any) we skip
// compression. If we enabled compression here we would always compress small
// error responses and send compression header even for empty responses.
func (w *responseWriter) WriteHeader(c int) {
	w.initCompressor(false)
	w.ResponseWriter.WriteHeader(c)
}

func (w *responseWriter) Flush() {
	if w.err != nil {
		return
	}

	// If there is anything in the buffer, pass to compressor
	if len(w.buf) > 0 {
		w.err = w.initCompressor(true)
	}

	if w.cw != nil {
		if err := w.cw.Flush(); err != nil {
			w.err = err
		}
	}

	if f, _ := w.ResponseWriter.(http.Flusher); f != nil {
		f.Flush()
	}
}

func (w *responseWriter) Close() {
	w.initCompressor(false)
	w.cw.Close()
}

func (w *responseWriter) detectContentType() {
	if w.buf != nil && w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", http.DetectContentType(w.buf))
	}
}

// create compressor, feed it with buffer;
// if ok is set to false we don't want to use compression.
// It's a no-op if compressor is already initialized.
func (w *responseWriter) initCompressor(ok bool) (err error) {
	if w.cw != nil {
		return nil
	}

	if ok {
		w.detectContentType()

		switch w.method {
		case encGzip:
			w.cw = gzip.NewWriter(w.ResponseWriter)
		case encDeflate:
			w.cw = zlib.NewWriter(w.ResponseWriter)
		default:
			panic(w.method)
		}

		// Set Content-Encoding and delete Content-Length as it gets invalidated
		w.Header().Set("Content-Encoding", string(w.method))
		w.Header().Del("Content-Length")
	} else {
		w.cw = noneCompressor{w.ResponseWriter}
	}

	// Don't write empty buffer as it would write a gzip header,
	// flushing the HTTP header onto the wire.
	if len(w.buf) > 0 {
		_, err = w.cw.Write(w.buf)
	}

	w.buf = nil
	return err
}

// compressor is a common interface for compressors. It's similar to
// writeFlusher, but flush returns error, which is ignored by this library.
type compressor interface {
	io.WriteCloser
	Flush() error
}

// noneCompressor implements compressor interface without
// performing any compression
type noneCompressor struct {
	io.Writer
}

func (noneCompressor) Close() error {
	return nil
}

func (noneCompressor) Flush() error {
	return nil
}
