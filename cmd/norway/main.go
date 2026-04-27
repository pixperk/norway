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
	"github.com/pixperk/norway/reload"
	"github.com/pixperk/norway/router"
	"github.com/pixperk/norway/stats"
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

	// initial build
	handler, checkers, collector := buildFromConfig(cfg)
	r := handler.(*router.Router)

	// set up hot reload
	swappable := reload.NewSwappableHandler(r)
	reloader := reload.New(*configPath, swappable, buildFromConfig)
	reloader.SetCheckers(checkers)

	// mount internal endpoints
	r.AddInternal("/norway/stats", collector.Handler())
	r.AddInternal("/norway/reload", reloader.APIHandler())

	// watch config file for changes
	reloader.WatchFile()

	// start a listener for each entrypoint, TLS or plain based on config
	var servers []*http.Server
	for _, ep := range cfg.Entrypoints {
		srv := &http.Server{Addr: ep.Listen, Handler: swappable}
		servers = append(servers, srv)

		go func(s *http.Server, ep dsl.Entrypoint) {
			if ep.TLS != nil {
				fmt.Printf("norway listening on %s (TLS)\n", s.Addr)
				err := s.ListenAndServeTLS(ep.TLS.CertPath, ep.TLS.KeyPath)
				if err != nil && err != http.ErrServerClosed {
					log.Fatalf("TLS server error on %s: %v", s.Addr, err)
				}
				return
			}

			fmt.Printf("norway listening on %s\n", s.Addr)
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("server error on %s: %v", s.Addr, err)
			}
		}(srv, ep)
	}

	// SIGINT/SIGTERM = shutdown, SIGHUP = reload
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	for s := range sig {
		if s == syscall.SIGHUP {
			log.Println("received SIGHUP, reloading config...")
			if err := reloader.Reload(); err != nil {
				log.Printf("reload failed: %v", err)
			}
			continue
		}

		// SIGINT or SIGTERM = shutdown
		log.Println("shutting down, draining connections...")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

		for _, srv := range servers {
			srv.Shutdown(ctx)
		}
		cancel()
		log.Println("norway stopped")
		return
	}
}

// buildFromConfig takes a parsed config and builds a router with balancers,
// health checkers, and a stats collector. Used on startup and on reload.
func buildFromConfig(cfg *dsl.Config) (http.Handler, []*health.Checker, *stats.Collector) {
	// index middlewares by name
	mwConfigs := make(map[string]dsl.Middleware)
	for _, mw := range cfg.Middlewares {
		mwConfigs[mw.Name] = mw
	}

	// build balancers and health checkers
	balancers := make(map[string]balance.Balancer)
	var checkers []*health.Checker

	for _, svc := range cfg.Services {
		var backends []*balance.Backend
		for _, srv := range svc.Servers {
			b, err := balance.NewBackend(srv.URL, srv.Weight)
			if err != nil {
				log.Printf("service %q: invalid server URL %q: %v", svc.Name, srv.URL, err)
				continue
			}
			backends = append(backends, b)
		}

		// if any backend has weight > 1, upgrade round-robin to weighted
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

		if svc.Health != nil {
			interval, _ := time.ParseDuration(svc.Health.Interval)
			timeout, _ := time.ParseDuration(svc.Health.Timeout)
			hc := health.New(backends, svc.Health.Path, interval, timeout)
			hc.Start()
			checkers = append(checkers, hc)
			log.Printf("service %q: health checks every %s on %s", svc.Name, svc.Health.Interval, svc.Health.Path)
		}
	}

	// stats collector with all backends
	var allBackends []*balance.Backend
	for _, bal := range balancers {
		allBackends = append(allBackends, bal.All()...)
	}
	collector := stats.NewCollector(allBackends)

	// build router
	r := router.New()
	for _, route := range cfg.Routes {
		bal, ok := balancers[route.Service]
		if !ok {
			log.Printf("route %q: service %q not found, skipping", route.Name, route.Service)
			continue
		}

		routeName := route.Name
		proxyHandler := balancedProxy(bal, collector, routeName)

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

	return r, checkers, collector
}

// balancedProxy returns a handler that picks a backend from the balancer,
// records stats, proxies the request, then cleans up.
func balancedProxy(bal balance.Balancer, collector *stats.Collector, routeName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		done := collector.RecordRequest(routeName)
		defer done()

		backend := bal.Next()
		if backend == nil {
			http.Error(w, "no healthy backends", http.StatusServiceUnavailable)
			return
		}

		backend.ActiveConns.Add(1)
		defer backend.ActiveConns.Add(-1)

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

// buildMiddlewares converts middleware names into actual Middleware functions
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
		case "ratelimit":
			rate := 100.0
			burst := 50
			if v, ok := mwCfg.Config["rate"]; ok {
				fmt.Sscanf(v, "%f", &rate)
			}
			if v, ok := mwCfg.Config["burst"]; ok {
				fmt.Sscanf(v, "%d", &burst)
			}
			mws = append(mws, middleware.RateLimit(rate, burst))
		case "https-redirect":
			mws = append(mws, middleware.HTTPSRedirect(mwCfg.Config["host"]))
		}
	}
	return mws
}

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
