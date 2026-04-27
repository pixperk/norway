package reload

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/pixperk/norway/dsl"
	"github.com/pixperk/norway/health"
	"github.com/pixperk/norway/router"
	"github.com/pixperk/norway/stats"
)

const benchConfig = `
entrypoint web {
    listen :8080
}

service api {
    balance round-robin
    server http://localhost:8001
    server http://localhost:8002
}

service app {
    balance least-conn
    server http://localhost:3000
}

middleware logger {
    type logging
    format json
}

route api {
    entrypoints web
    host api.example.com
    path /v1/*
    service api
    use logger
}

route app {
    entrypoints web
    host app.example.com
    service app
    use logger
}
`

// minimalBuild is a stripped-down BuildFunc for benchmarking.
// It avoids starting health checkers (no goroutines to leak in the bench loop).
func minimalBuild(cfg *dsl.Config) (http.Handler, []*health.Checker, *stats.Collector) {
	r := router.New()
	for _, route := range cfg.Routes {
		if route.Host == "" {
			continue
		}
		path := route.Path
		if path == "" {
			path = "/"
		}
		r.Add(route.Host, path, http.NotFoundHandler())
	}
	return r, nil, stats.NewCollector(nil)
}

// BenchmarkReload measures the full reload cycle: read file, lex, parse,
// validate, build router, atomic swap. This is what runs on every fsnotify
// event, SIGHUP, or POST /norway/reload.
func BenchmarkReload(b *testing.B) {
	dir := b.TempDir()
	cfgPath := filepath.Join(dir, "norway.conf")
	if err := os.WriteFile(cfgPath, []byte(benchConfig), 0644); err != nil {
		b.Fatal(err)
	}

	swappable := NewSwappableHandler(http.NotFoundHandler())
	rl := New(cfgPath, swappable, minimalBuild)
	rl.debounce = 0 // disable debounce for the bench

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := rl.Reload(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDSLPipeline measures lex + parse + validate without file I/O.
// This is the compute portion of every reload, useful to know in isolation.
func BenchmarkDSLPipeline(b *testing.B) {
	src := benchConfig

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tokens := dsl.NewLexer(src).Tokenize()
		cfg, err := dsl.NewParser(tokens).Parse()
		if err != nil {
			b.Fatal(err)
		}
		if err := dsl.Validate(cfg); err != nil {
			b.Fatal(err)
		}
	}
}
