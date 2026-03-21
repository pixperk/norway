package dsl

import (
	"strings"
	"testing"
)

func validConfig() *Config {
	return &Config{
		Entrypoints: []Entrypoint{
			{Name: "web", Listen: ":80"},
		},
		Services: []Service{
			{Name: "api", Balance: "round-robin", Servers: []Server{{URL: "http://localhost:8001", Weight: 1}}},
		},
		Middlewares: []Middleware{
			{Name: "logger", Type: "logging", Config: map[string]string{}},
		},
		Routes: []Route{
			{Name: "api", Entrypoints: []string{"web"}, Host: "api.example.com", Path: "/v1/*", Service: "api", Middlewares: []string{"logger"}},
		},
	}
}

func TestValidConfigPasses(t *testing.T) {
	if err := Validate(validConfig()); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}

func TestValidateNorwayConf(t *testing.T) {
	cfg := loadConf(t)
	if err := Validate(cfg); err != nil {
		t.Fatalf("norway.conf should be valid, got: %v", err)
	}
}

func TestNoEntrypoints(t *testing.T) {
	cfg := validConfig()
	cfg.Entrypoints = nil
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "at least 1 entrypoint") {
		t.Errorf("expected entrypoint error, got: %v", err)
	}
}

func TestNoServices(t *testing.T) {
	cfg := validConfig()
	cfg.Services = nil
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "at least 1 service") {
		t.Errorf("expected service error, got: %v", err)
	}
}

func TestNoRoutes(t *testing.T) {
	cfg := validConfig()
	cfg.Routes = nil
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "at least 1 route") {
		t.Errorf("expected route error, got: %v", err)
	}
}

func TestDuplicateEntrypoint(t *testing.T) {
	cfg := validConfig()
	cfg.Entrypoints = append(cfg.Entrypoints, Entrypoint{Name: "web", Listen: ":81"})
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "duplicate entrypoint") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

func TestDuplicateService(t *testing.T) {
	cfg := validConfig()
	cfg.Services = append(cfg.Services, Service{Name: "api", Balance: "round-robin", Servers: []Server{{URL: "http://localhost:8002", Weight: 1}}})
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "duplicate service") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

func TestDuplicateMiddleware(t *testing.T) {
	cfg := validConfig()
	cfg.Middlewares = append(cfg.Middlewares, Middleware{Name: "logger", Type: "logging", Config: map[string]string{}})
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "duplicate middleware") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

func TestDuplicateRoute(t *testing.T) {
	cfg := validConfig()
	cfg.Routes = append(cfg.Routes, Route{Name: "api", Entrypoints: []string{"web"}, Service: "api"})
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "duplicate route") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

func TestMissingListen(t *testing.T) {
	cfg := validConfig()
	cfg.Entrypoints[0].Listen = ""
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "missing listen") {
		t.Errorf("expected listen error, got: %v", err)
	}
}

func TestTLSMissingCert(t *testing.T) {
	cfg := validConfig()
	cfg.Entrypoints[0].TLS = &TLSConfig{KeyPath: "/key.pem"}
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "tls missing cert") {
		t.Errorf("expected tls cert error, got: %v", err)
	}
}

func TestTLSMissingKey(t *testing.T) {
	cfg := validConfig()
	cfg.Entrypoints[0].TLS = &TLSConfig{CertPath: "/cert.pem"}
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "tls missing key") {
		t.Errorf("expected tls key error, got: %v", err)
	}
}

func TestServiceNoServers(t *testing.T) {
	cfg := validConfig()
	cfg.Services[0].Servers = nil
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "at least 1 server") {
		t.Errorf("expected server error, got: %v", err)
	}
}

func TestUnknownBalanceStrategy(t *testing.T) {
	cfg := validConfig()
	cfg.Services[0].Balance = "random"
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "unknown balance strategy") {
		t.Errorf("expected strategy error, got: %v", err)
	}
}

func TestRouteUnknownService(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].Service = "nonexistent"
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "unknown service") {
		t.Errorf("expected unknown service error, got: %v", err)
	}
}

func TestRouteUnknownEntrypoint(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].Entrypoints = []string{"nonexistent"}
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "unknown entrypoint") {
		t.Errorf("expected unknown entrypoint error, got: %v", err)
	}
}

func TestRouteUnknownMiddleware(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].Middlewares = []string{"nonexistent"}
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "unknown middleware") {
		t.Errorf("expected unknown middleware error, got: %v", err)
	}
}

func TestRouteMissingService(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].Service = ""
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "missing service") {
		t.Errorf("expected missing service error, got: %v", err)
	}
}

func TestRouteMissingEntrypoints(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].Entrypoints = nil
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "at least 1 entrypoint") {
		t.Errorf("expected entrypoint error, got: %v", err)
	}
}
