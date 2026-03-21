package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/pixperk/norway/dsl"
	"github.com/pixperk/norway/router"
)

func main() {
	configPath := flag.String("config", "norway.conf", "path to config file")
	flag.Parse()

	// read and parse config
	data, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("failed to read config: %v", err)
	}

	tokens := dsl.NewLexer(string(data)).Tokenize()
	cfg, err := dsl.NewParser(tokens).Parse()
	if err != nil {
		log.Fatalf("config parse error: %v", err)
	}

	if err := dsl.Validate(cfg); err != nil {
		log.Fatalf("config validation error: %v", err)
	}

	// build service proxies: service name -> reverse proxy
	proxies := make(map[string]*httputil.ReverseProxy)
	for _, svc := range cfg.Services {
		// for now use the first server in each service
		// todo : support multiple servers per service with load balancing
		target, err := url.Parse(svc.Servers[0].URL)
		if err != nil {
			log.Fatalf("service %q: invalid server URL %q: %v", svc.Name, svc.Servers[0].URL, err)
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Transport = &http.Transport{
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error: %s %s -> %v", r.Method, r.URL.Path, err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		}
		proxies[svc.Name] = proxy
	}

	// build router from routes
	r := router.New()
	for _, route := range cfg.Routes {
		proxy, ok := proxies[route.Service]
		if !ok {
			log.Fatalf("route %q: service %q not found", route.Name, route.Service)
		}

		// wrap proxy with request logging
		handler := loggingHandler(route.Name, proxy)

		path := route.Path
		if path == "" {
			path = "/"
		}

		if route.Host != "" {
			r.Add(route.Host, path, handler)
		}
	}

	// start a listener for each entrypoint
	for i, ep := range cfg.Entrypoints {
		addr := ep.Listen
		if i < len(cfg.Entrypoints)-1 {
			// all but the last entrypoint run in a goroutine
			go func(addr string) {
				fmt.Printf("norway listening on %s\n", addr)
				if err := http.ListenAndServe(addr, r); err != nil {
					log.Fatalf("server error on %s: %v", addr, err)
				}
			}(addr)
		} else {
			// last one blocks
			fmt.Printf("norway listening on %s\n", addr)
			if err := http.ListenAndServe(addr, r); err != nil {
				log.Fatalf("server error on %s: %v", addr, err)
			}
		}
	}
}

func loggingHandler(routeName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("-> %s %s%s [route:%s]", r.Method, r.Host, r.URL.Path, routeName)
		next.ServeHTTP(w, r)
		log.Printf("<- %s %s%s [route:%s] (%s)", r.Method, r.Host, r.URL.Path, routeName, time.Since(start))
	})
}
