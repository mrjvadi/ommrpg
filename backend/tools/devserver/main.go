// Command devserver serves the exported web client and proxies the API and
// the Centrifugo websocket on one origin, like deploy/nginx does in
// production. For local development without Docker.
//
//	go run ./tools/devserver -web ../client/build/web -addr :8088
package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func main() {
	web := flag.String("web", "../client/build/web", "exported Godot web build")
	addr := flag.String("addr", ":8088", "listen address")
	api := flag.String("api", "http://localhost:8080", "gateway")
	rt := flag.String("realtime", "http://localhost:8000", "centrifugo")
	flag.Parse()
	gw, _ := url.Parse(*api)
	cf, _ := url.Parse(*rt)
	apiProxy := httputil.NewSingleHostReverseProxy(gw)
	rtProxy := httputil.NewSingleHostReverseProxy(cf)
	files := http.FileServer(http.Dir(*web))
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/"):
			apiProxy.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/connection/"):
			rtProxy.ServeHTTP(w, r)
		default:
			if strings.HasSuffix(r.URL.Path, ".wasm") {
				w.Header().Set("Content-Type", "application/wasm")
			}
			files.ServeHTTP(w, r)
		}
	})
	log.Printf("serving %s on %s", *web, *addr)
	log.Fatal(http.ListenAndServe(*addr, h))
}
