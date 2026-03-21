package dsl

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func loadConf(t *testing.T) *Config {
	t.Helper()
	// find norway.conf relative to this test file
	_, filename, _, _ := runtime.Caller(0)
	confPath := filepath.Join(filepath.Dir(filename), "..", "norway.conf")

	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("failed to read norway.conf: %v", err)
	}

	tokens := NewLexer(string(data)).Tokenize()
	parser := NewParser(tokens)
	cfg, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return cfg
}

func TestParseEntrypoints(t *testing.T) {
	cfg := loadConf(t)

	if len(cfg.Entrypoints) != 2 {
		t.Fatalf("expected 2 entrypoints, got %d", len(cfg.Entrypoints))
	}

	web := cfg.Entrypoints[0]
	if web.Name != "web" || web.Listen != ":80" {
		t.Errorf("web entrypoint: got name=%q listen=%q", web.Name, web.Listen)
	}
	if web.TLS != nil {
		t.Error("web entrypoint should not have TLS")
	}

	websecure := cfg.Entrypoints[1]
	if websecure.Name != "websecure" || websecure.Listen != ":443" {
		t.Errorf("websecure entrypoint: got name=%q listen=%q", websecure.Name, websecure.Listen)
	}
	if websecure.TLS == nil {
		t.Fatal("websecure entrypoint should have TLS")
	}
	if websecure.TLS.CertPath != "/etc/norway/cert.pem" {
		t.Errorf("tls cert: got %q", websecure.TLS.CertPath)
	}
	if websecure.TLS.KeyPath != "/etc/norway/key.pem" {
		t.Errorf("tls key: got %q", websecure.TLS.KeyPath)
	}
}

func TestParseServices(t *testing.T) {
	cfg := loadConf(t)

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}

	api := cfg.Services[0]
	if api.Name != "api" || api.Balance != "round-robin" {
		t.Errorf("api service: got name=%q balance=%q", api.Name, api.Balance)
	}
	if api.Health == nil {
		t.Fatal("api service should have health config")
	}
	if api.Health.Path != "/health" || api.Health.Interval != "10s" || api.Health.Timeout != "2s" {
		t.Errorf("health: got path=%q interval=%q timeout=%q", api.Health.Path, api.Health.Interval, api.Health.Timeout)
	}
	if len(api.Servers) != 2 {
		t.Fatalf("api: expected 2 servers, got %d", len(api.Servers))
	}
	if api.Servers[0].URL != "http://localhost:8001" || api.Servers[0].Weight != 3 {
		t.Errorf("server 0: got url=%q weight=%d", api.Servers[0].URL, api.Servers[0].Weight)
	}
	if api.Servers[1].URL != "http://localhost:8002" || api.Servers[1].Weight != 1 {
		t.Errorf("server 1: got url=%q weight=%d", api.Servers[1].URL, api.Servers[1].Weight)
	}

	app := cfg.Services[1]
	if app.Name != "app" || app.Balance != "least-conn" {
		t.Errorf("app service: got name=%q balance=%q", app.Name, app.Balance)
	}
	if len(app.Servers) != 1 || app.Servers[0].URL != "http://localhost:3000" {
		t.Errorf("app servers: got %+v", app.Servers)
	}
}

func TestParseMiddlewares(t *testing.T) {
	cfg := loadConf(t)

	if len(cfg.Middlewares) != 3 {
		t.Fatalf("expected 3 middlewares, got %d", len(cfg.Middlewares))
	}

	rl := cfg.Middlewares[0]
	if rl.Name != "rate-limit" || rl.Type != "ratelimit" {
		t.Errorf("rate-limit: got name=%q type=%q", rl.Name, rl.Type)
	}
	if rl.Config["rate"] != "100" || rl.Config["burst"] != "50" {
		t.Errorf("rate-limit config: got %v", rl.Config)
	}

	hd := cfg.Middlewares[1]
	if hd.Name != "add-headers" || hd.Type != "headers" {
		t.Errorf("add-headers: got name=%q type=%q", hd.Name, hd.Type)
	}

	logger := cfg.Middlewares[2]
	if logger.Name != "logger" || logger.Type != "logging" {
		t.Errorf("logger: got name=%q type=%q", logger.Name, logger.Type)
	}
	if logger.Config["format"] != "json" {
		t.Errorf("logger config: got %v", logger.Config)
	}
}

func TestParseRoutes(t *testing.T) {
	cfg := loadConf(t)

	if len(cfg.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(cfg.Routes))
	}

	api := cfg.Routes[0]
	if api.Name != "api" {
		t.Errorf("route name: got %q", api.Name)
	}
	if len(api.Entrypoints) != 2 || api.Entrypoints[0] != "web" || api.Entrypoints[1] != "websecure" {
		t.Errorf("api entrypoints: got %v", api.Entrypoints)
	}
	if api.Host != "api.example.com" {
		t.Errorf("api host: got %q", api.Host)
	}
	if api.Path != "/v1/*" {
		t.Errorf("api path: got %q", api.Path)
	}
	if api.Service != "api" {
		t.Errorf("api service: got %q", api.Service)
	}
	if len(api.Middlewares) != 3 {
		t.Fatalf("api: expected 3 middlewares, got %d", len(api.Middlewares))
	}
	if api.Middlewares[0] != "rate-limit" || api.Middlewares[1] != "add-headers" || api.Middlewares[2] != "logger" {
		t.Errorf("api middlewares: got %v", api.Middlewares)
	}

	app := cfg.Routes[1]
	if app.Name != "app" || app.Host != "app.example.com" || app.Service != "app" {
		t.Errorf("app route: got name=%q host=%q service=%q", app.Name, app.Host, app.Service)
	}
	if len(app.Middlewares) != 1 || app.Middlewares[0] != "logger" {
		t.Errorf("app middlewares: got %v", app.Middlewares)
	}
}

func TestParseError(t *testing.T) {
	input := `entrypoint web {
    listen :80
    unknown_thing value
}`
	tokens := NewLexer(input).Tokenize()
	_, err := NewParser(tokens).Parse()
	if err == nil {
		t.Fatal("expected error for unknown directive")
	}
}
