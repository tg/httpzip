package httpzip

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
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

func TestSameHeaders(t *testing.T) {
	// Default server without wrappers, manual compression
	s1 := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Content-Encoding", "gzip")
			cw := gzip.NewWriter(w)
			io.Copy(cw, r.Body)
			cw.Close()
		}))
	defer s1.Close()

	// Server with the wrapper
	s2 := httptest.NewServer(NewHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.Copy(w, r.Body)
		})))
	defer s2.Close()

	getResponse := func(url string) (string, error) {
		req, _ := http.NewRequest("POST", url, strings.NewReader("text"))
		req.Header.Set("Accept-Encoding", "gzip")

		r, err := new(http.Client).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if r.StatusCode != http.StatusOK {
			t.Fatal(r.Status)
		}
		body, _ := ioutil.ReadAll(r.Body)
		r.Body.Close()
		d, err := httputil.DumpResponse(r, false)
		return fmt.Sprintf("%s\n%s", d, body), err
	}

	r1, err := getResponse(s1.URL)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := getResponse(s2.URL)
	if err != nil {
		t.Fatal(err)
	}

	if r1 != r2 {
		t.Errorf("Data mismatch:\n---expected---\n%s\n---received---\n%s", r1, r2)
	}
}
