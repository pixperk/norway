package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func main() {
	listenAddr := flag.String("listen", ":8080", "address to listen on")
	backend := flag.String("backend", "http://localhost:3000", "backend URL to proxy to")
	flag.Parse()

	target, err := url.Parse(*backend)
	if err != nil {
		log.Fatalf("invalid backend URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Transport = &http.Transport{
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error: %s %s → %v", r.Method, r.URL.Path, err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("→ %s %s%s", r.Method, r.Host, r.URL.Path)
		proxy.ServeHTTP(w, r)
		log.Printf("← %s %s%s (%s)", r.Method, r.Host, r.URL.Path, time.Since(start))
	})

	fmt.Printf("norway listening on %s → proxying to %s\n", *listenAddr, target)
	if err := http.ListenAndServe(*listenAddr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
