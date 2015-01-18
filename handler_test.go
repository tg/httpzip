package httpzip

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/quick"
)

func TestTwoWay(t *testing.T) {
	test := func(data string) bool {
		uncompressed := &bytes.Buffer{}
		h := NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Store data in uncompressed as well as pass to the response
			io.Copy(io.MultiWriter(uncompressed, w), r.Body)
		}))

		payload := &bytes.Buffer{}
		w := gzip.NewWriter(payload)
		w.Write([]byte(data))
		w.Close()
		compressed := payload.String()
		req, _ := http.NewRequest("POST", "http://test.com", payload)
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		// Check we've encoded and decoded data
		return uncompressed.String() == data && rr.Body.String() == compressed
	}

	if err := quick.Check(test, nil); err != nil {
		t.Fatal(err)
	}
}
