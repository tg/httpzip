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
	h := NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(w, r.Body)
	}))

	f := func(data string) bool {
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
		return rr.Body.String() == compressed
	}

	if err := quick.Check(f, nil); err != nil {
		t.Fatal(err)
	}
}
