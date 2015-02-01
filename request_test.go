package httpzip

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestRequestHandler(t *testing.T) {
	h := NewRequestHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if _, err := io.Copy(w, r.Body); err != nil {
			t.Fatal(err)
		}
	}))

	data := "some uncompressed data"
	usegzip := func(r io.Writer) (io.WriteCloser, error) { return gzip.NewWriter(r), nil }
	useflate := func(r io.Writer) (io.WriteCloser, error) { return flate.NewWriter(r, flate.DefaultCompression) }

	tests := []struct {
		content string
		writer  func(r io.Writer) (io.WriteCloser, error)
	}{
		{"gzip", usegzip},
		{"deflate", useflate},
	}

	for _, test := range tests {
		t.Log("Testing", test.content)

		compressed, size := func() (io.Reader, int) {
			b := &bytes.Buffer{}
			w, err := test.writer(b)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			if _, err := strings.NewReader(data).WriteTo(w); err != nil {
				t.Fatal(err)
			}
			return b, b.Len()
		}()

		req, _ := http.NewRequest("GET", "http://test.com", compressed)
		req.Header.Set("Content-Encoding", test.content)
		req.Header.Set("Content-Length", strconv.Itoa(size))

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		// Check request header modifications
		htest := map[string]string{
			"Content-Encoding": test.content,
			"Content-Length":   strconv.Itoa(size),
		}
		for k, orgv := range htest {
			if v := req.Header.Get(k); v != "" {
				t.Errorf("Unexpected header %s: %s", k, v)
			}
			org := "X-Original-" + k
			if v := req.Header.Get(org); v == "" {
				t.Error("Expected header:", org)
			} else if v != orgv {
				t.Errorf("%s expected value %q, has %q", org, orgv, v)
			}
		}

		if rr.Code != http.StatusOK {
			t.Fatal(rr.Code, rr.Body.String())
		}
		if d := rr.Body.String(); d != data {
			t.Fatalf("Expected %q, got %q", data, d)
		}
	}
}

func TestGzipErrHeader(t *testing.T) {
	var herr error
	h := NewRequestHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_, herr = ioutil.ReadAll(r.Body)
	}))

	req, _ := http.NewRequest("GET", "http://test.com", strings.NewReader("wrong-gzip"))
	req.Header.Set("Content-Encoding", "gzip")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if herr != gzip.ErrHeader {
		t.Fatal(herr)
	}
}
