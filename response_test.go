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
	h := NewResponseHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(w, r.Body); err != nil {
			t.Fatal(err)
		}
	}))
	hf := NewResponseHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(w, r.Body); err != nil {
			t.Fatal(err)
		}
		w.(http.Flusher).Flush()
	}))

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
		flash  bool

		// output
		content string
		uncompr func(r io.Reader) (io.ReadCloser, error)
	}{
		{bigdata, "gzip", true, "gzip", usegzip},
		{bigdata, "deflate", true, "deflate", useflate},

		{bigdata, "gzip, deflate", true, "gzip", usegzip},
		{bigdata, "deflate, gzip", true, "gzip", usegzip},

		{bigdata, "deflate, gzip, unknown", true, "gzip", usegzip},

		{smalldata, "gzip", false, "", usenothing},
		{smalldata, "gzip", true, "gzip", usegzip},

		{"", "gzip", true, "", usenothing},
		{"", "gzip", false, "", usenothing},

		{smalldata, "", true, "", usenothing},
	}

	for _, test := range tests {
		t.Log("Checking Accept-Encoding:", test.accept)

		req, _ := http.NewRequest("GET", "http://test.com", strings.NewReader(test.data))
		req.Header.Set("Accept-Encoding", test.accept)

		rr := httptest.NewRecorder()
		if test.flash {
			hf.ServeHTTP(rr, req)
			if !rr.Flushed {
				t.Error("Response not flushed")
			}
		} else {
			h.ServeHTTP(rr, req)
		}
		if rr.Code != http.StatusOK {
			t.Error(rr.Code, rr.Body.String())
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
				t.Fatalf("Expected %q, got %q", test.data, string(got))
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
