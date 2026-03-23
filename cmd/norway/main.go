package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pixperk/norway/balance"
	"github.com/pixperk/norway/dsl"
	"github.com/pixperk/norway/health"
	"github.com/pixperk/norway/middleware"
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

	// index middlewares by name for quick lookup
	mwConfigs := make(map[string]dsl.Middleware)
	for _, mw := range cfg.Middlewares {
		mwConfigs[mw.Name] = mw
	}

	// build a balancer for each service from its servers + strategy
	balancers := make(map[string]balance.Balancer)
	for _, svc := range cfg.Services {
		var backends []*balance.Backend
		for _, srv := range svc.Servers {
			b, err := balance.NewBackend(srv.URL, srv.Weight)
			if err != nil {
				log.Fatalf("service %q: invalid server URL %q: %v", svc.Name, srv.URL, err)
			}
			backends = append(backends, b)
		}

		// if any backend has weight > 1, upgrade round-robin to weighted automatically
		hasWeights := false
		for _, b := range backends {
			if b.Weight > 1 {
				hasWeights = true
				break
			}
		}

		switch svc.Balance {
		case "weighted":
			balancers[svc.Name] = balance.NewWeighted(backends)
		case "least-conn":
			balancers[svc.Name] = balance.NewLeastConn(backends)
		default:
			if hasWeights {
				balancers[svc.Name] = balance.NewWeighted(backends)
			} else {
				balancers[svc.Name] = balance.NewRoundRobin(backends)
			}
		}

		log.Printf("service %q: %d backends, strategy=%s", svc.Name, len(backends), svc.Balance)

		// start health checker if configured
		if svc.Health != nil {
			interval, _ := time.ParseDuration(svc.Health.Interval)
			timeout, _ := time.ParseDuration(svc.Health.Timeout)
			hc := health.New(backends, svc.Health.Path, interval, timeout)
			hc.Start()
			log.Printf("service %q: health checks every %s on %s", svc.Name, svc.Health.Interval, svc.Health.Path)
		}
	}

	// build router from routes
	r := router.New()
	for _, route := range cfg.Routes {
		bal, ok := balancers[route.Service]
		if !ok {
			log.Fatalf("route %q: service %q not found", route.Name, route.Service)
		}

		// the handler picks a backend via the balancer and proxies to it
		proxyHandler := balancedProxy(bal)

		// build middleware chain for this route from config
		mws := buildMiddlewares(route.Middlewares, mwConfigs)
		handler := middleware.Chain(proxyHandler, mws...)

		path := route.Path
		if path == "" {
			path = "/"
		}

		if route.Host != "" {
			r.Add(route.Host, path, handler)
		}
	}

	// start a listener for each entrypoint
	var servers []*http.Server
	for _, ep := range cfg.Entrypoints {
		srv := &http.Server{Addr: ep.Listen, Handler: r}
		servers = append(servers, srv)

		go func(s *http.Server) {
			fmt.Printf("norway listening on %s\n", s.Addr)
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("server error on %s: %v", s.Addr, err)
			}
		}(srv)
	}

	// graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("shutting down, draining connections...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for _, srv := range servers {
		srv.Shutdown(ctx)
	}
	log.Println("norway stopped")
}

// balancedProxy returns a handler that picks a backend from the balancer on each request,
// increments active connections, proxies, then decrements.
func balancedProxy(bal balance.Balancer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend := bal.Next()
		if backend == nil {
			http.Error(w, "no healthy backends", http.StatusServiceUnavailable)
			return
		}

		// track active connections for least-conn
		backend.ActiveConns.Add(1)
		defer backend.ActiveConns.Add(-1)

		// timeout so slow backends don't hang forever
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		proxy := httputil.NewSingleHostReverseProxy(backend.URL)
		proxy.Transport = backend.Transport
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error: %s %s -> %s: %v", r.Method, r.URL.Path, backend.URL, err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		}

		proxy.ServeHTTP(w, r)
	})
}

// buildMiddlewares converts middleware names from the route config into actual Middleware functions
func buildMiddlewares(names []string, configs map[string]dsl.Middleware) []middleware.Middleware {
	var mws []middleware.Middleware
	for _, name := range names {
		mwCfg, ok := configs[name]
		if !ok {
			continue
		}
		switch mwCfg.Type {
		case "logging":
			mws = append(mws, middleware.Logging())
		case "headers":
			add, remove := parseHeadersConfig(mwCfg.Config)
			mws = append(mws, middleware.Headers(add, remove))
		}
	}
	return mws
}

// parseHeadersConfig extracts add/remove maps from the middleware config.
// In the DSL, "set X-Proxy" maps to key "set X-Proxy" with value "norway",
// and "remove" maps to key "remove" with the header name as value.
func parseHeadersConfig(config map[string]string) (add map[string]string, remove []string) {
	add = make(map[string]string)
	for key, val := range config {
		if strings.HasPrefix(key, "set ") {
			headerName := strings.TrimPrefix(key, "set ")
			add[headerName] = val
		}
		if key == "remove" {
			remove = append(remove, val)
		}
	}
	return add, remove
}
