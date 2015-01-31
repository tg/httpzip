# httpzip [![GoDoc](https://godoc.org/github.com/tg/httpzip?status.svg)](https://godoc.org/github.com/tg/httpzip) [![Build Status](https://travis-ci.org/tg/httpzip.svg?branch=master)](https://travis-ci.org/tg/httpzip)
Transparently decompress http.Server requests and compress responses with gzip and deflate.

## What do you get?
Contrary to many cheap solutions you can find on Q&A sites, this library gives you:
- Both compressing and decompressing wrappers for `http.Handler`
- No compression for responses under 512 bytes
- `http.ResponseWriter` using [`http.DetectContentType`](http://golang.org/pkg/net/http/#DetectContentType) on a full 512-byte chunk of initial uncompressed data (not only the first written chunk)
- `http.ResponseWriter` implementing [`http.Flusher`](http://golang.org/pkg/net/http/#Flusher), preserving [`http.CloseNotifier`](http://golang.org/pkg/net/http/#CloseNotifier) and [`http.Hijacker`](http://golang.org/pkg/net/http/#Hijacker) interfaces
- No empty archives being sent on responses with no body

## Example
```go
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/tg/httpzip"
)

func main() {
	// Handler reads and writes uncompressed data, as usual
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		log.Printf("%q (err: %v)", body, err)
		fmt.Fprint(w, "understood!")
	})

	// httpzip handlers will transparently (de)compress data
	http.Handle("/nothing", h)
	http.Handle("/compress", httpzip.NewResponseHandler(h))
	http.Handle("/decompress", httpzip.NewRequestHandler(h))
	http.Handle("/both", httpzip.NewHandler(h))

	log.Fatal(http.ListenAndServe(":8080", nil))

	// Or you can wrap your ServeMux to enable (de)compression for all handlers at once:
	// http.ListenAndServe(":8080", httpzip.NewHandler(http.DefaultServeMux))
}
```
