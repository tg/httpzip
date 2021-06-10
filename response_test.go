package httpzip

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInterfaces(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v, ok := w.(*responseWriter); !ok || v == nil {
			t.Fatal("ResponseWriter not wrapped?")
		}
		if v, ok := w.(http.CloseNotifier); !ok || v == nil {
			t.Fatal("ResponseWriter doesn't implement CloseNotifier")
		}
		if v, ok := w.(http.Hijacker); !ok || v == nil {
			t.Fatal("ResponseWriter doesn't implement Hijacker")
		}
		if v, ok := w.(http.Flusher); !ok || v == nil {
			t.Fatal("ResponseWriter doesn't implement Flusher")
		}
	})
	s := httptest.NewServer(NewResponseHandler(h))
	defer s.Close()

	r, err := http.Get(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != http.StatusOK {
		t.Fatal(r.Status)
	}
}

func TestResponseHandler(t *testing.T) {
	smalldata := "0123456789"
	// Big data should be over compression threshold
	bigdata := strings.Repeat(smalldata, initBufferSize/len(smalldata)+1)

	usegzip := func(r io.Reader) (io.ReadCloser, error) { return gzip.NewReader(r) }
	useflate := func(r io.Reader) (io.ReadCloser, error) { return zlib.NewReader(r) }
	usenothing := func(r io.Reader) (io.ReadCloser, error) { return ioutil.NopCloser(r), nil }

	tests := []struct {
		// input
		data   string
		accept string

		// output
		content string
		uncompr func(r io.Reader) (io.ReadCloser, error)

		flash bool

		// if writeHeader is set we shouldn't use compression
		writeHeader int
	}{
		{bigdata, "gzip", "gzip", usegzip, true, 0},
		{bigdata, "deflate", "deflate", useflate, true, 0},

		{bigdata, "gzip, deflate", "gzip", usegzip, true, 0},
		{bigdata, "deflate, gzip", "gzip", usegzip, true, 0},

		{bigdata, "deflate, gzip, unknown", "gzip", usegzip, true, 0},

		{smalldata, "gzip", "", usenothing, false, 0},
		{smalldata, "gzip", "gzip", usegzip, true, 0},

		{"", "gzip", "", usenothing, true, 0},
		{"", "gzip", "", usenothing, false, 0},

		{smalldata, "", "", usenothing, true, 0},
		{bigdata, "", "", usenothing, true, 0},
		{bigdata, "unknown", "", usenothing, false, 0},

		// check no compression on status codes
		{bigdata, "gzip", "", usenothing, false, http.StatusBadRequest},
		{bigdata, "gzip", "", usenothing, true, http.StatusBadRequest},
		{bigdata, "gzip", "", usenothing, false, http.StatusOK},
		{smalldata, "gzip", "", usenothing, false, http.StatusBadRequest},
		{"", "gzip", "", usenothing, false, http.StatusBadRequest},
	}

	for testN, test := range tests {
		t.Log("Checking Accept-Encoding:", test.accept)

		req, _ := http.NewRequest("GET", "http://test.com", strings.NewReader(test.data))
		req.Header.Set("Accept-Encoding", test.accept)

		rr := httptest.NewRecorder()

		h := NewResponseHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if code := test.writeHeader; code != 0 {
				w.WriteHeader(code)
			}

			if _, err := io.Copy(w, r.Body); err != nil {
				t.Fatal(err)
			}

			if test.flash {
				w.(http.Flusher).Flush()
			}
		}))

		h.ServeHTTP(rr, req)

		if test.flash {
			if !rr.Flushed {
				t.Error("Response not flushed")
			}
		}

		expectedStatusCode := http.StatusOK
		if test.writeHeader != 0 {
			expectedStatusCode = test.writeHeader
		}

		if rr.Code != expectedStatusCode {
			t.Error(rr.Code, expectedStatusCode, rr.Body.String())
		}
		req.Body.Close()

		if ce := rr.Header().Get("Content-Encoding"); ce != test.content {
			t.Errorf("Content-Encoding: %v", ce)
		}

		// Uncompress response and compare with input
		func() {
			unc, err := test.uncompr(rr.Body)
			if err != nil {
				t.Fatal(err)
			}
			defer unc.Close()
			if got, err := ioutil.ReadAll(unc); err != nil {
				t.Fatal(err)
			} else if string(got) != test.data {
				t.Errorf("[%d] Expected %q, got %q", testN, test.data, string(got))
			}
		}()
	}
}

func TestResponseStatusCode(t *testing.T) {
	s := httptest.NewServer(NewResponseHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
			w.Write([]byte("data"))
		})))
	defer s.Close()

	req, _ := http.NewRequest("GET", s.URL, nil)
	r, err := new(http.Client).Do(req)
	if err != nil {
		t.Fatal(err)
	}

	if r.StatusCode != http.StatusTeapot {
		t.Errorf("Expected code %d, got %d", http.StatusTeapot, r.StatusCode)
	}
}
