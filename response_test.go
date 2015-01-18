package httpzip

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloseNotifier(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.CloseNotifier); !ok {
			t.Fatal("ResponseWriter doesn't implement CloseNotifier")
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

	data := "some uncompressed data"
	usegzip := func(r io.Reader) (io.ReadCloser, error) { return gzip.NewReader(r) }
	useflate := func(r io.Reader) (io.ReadCloser, error) { return flate.NewReader(r), nil }

	tests := []struct {
		accept  string
		content string
		uncompr func(r io.Reader) (io.ReadCloser, error)
	}{
		{"gzip", "gzip", usegzip},
		{"deflate", "deflate", useflate},

		{"gzip, deflate", "gzip", usegzip},
		{"deflate, gzip", "gzip", usegzip},

		{"deflate, gzip, unknown", "gzip", usegzip},
	}

	for _, test := range tests {
		t.Log("Checking Accept-Encoding:", test.accept)

		req, _ := http.NewRequest("GET", "http://test.com", strings.NewReader(data))
		req.Header.Set("Accept-Encoding", test.accept)

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatal(rr.Code, rr.Body.String())
		}

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
			} else if string(got) != data {
				t.Fatalf("Expected %q, got %q", data, string(got))
			}
		}()
	}
}
