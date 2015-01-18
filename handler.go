package httpzip

import "net/http"

// NewHandler wraps http handler to support compression for both requests
// and responses.
func NewHandler(h http.Handler) http.Handler {
	return NewResponseHandler(NewRequestHandler(h))
}
