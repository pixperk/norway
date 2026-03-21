package dsl

import "fmt"

var validStrategies = map[string]bool{
	"round-robin": true,
	"least-conn":  true,
	"weighted":    true,
}

// Validate checks semantic correctness of a parsed Config.
// The parser only checks syntax (valid DSL), this checks logic (does it make sense).
func Validate(cfg *Config) error {
	// check that config is not empty
	if len(cfg.Entrypoints) == 0 {
		return fmt.Errorf("config must have at least 1 entrypoint")
	}
	if len(cfg.Services) == 0 {
		return fmt.Errorf("config must have at least 1 service")
	}
	if len(cfg.Routes) == 0 {
		return fmt.Errorf("config must have at least 1 route")
	}

	// build name sets for each block type and check for duplicate names
	entrypoints := make(map[string]bool)
	for _, ep := range cfg.Entrypoints {
		if entrypoints[ep.Name] {
			return fmt.Errorf("duplicate entrypoint name %q", ep.Name)
		}
		entrypoints[ep.Name] = true
	}

	services := make(map[string]bool)
	for _, svc := range cfg.Services {
		if services[svc.Name] {
			return fmt.Errorf("duplicate service name %q", svc.Name)
		}
		services[svc.Name] = true
	}

	middlewares := make(map[string]bool)
	for _, mw := range cfg.Middlewares {
		if middlewares[mw.Name] {
			return fmt.Errorf("duplicate middleware name %q", mw.Name)
		}
		middlewares[mw.Name] = true
	}

	routes := make(map[string]bool)
	for _, rt := range cfg.Routes {
		if routes[rt.Name] {
			return fmt.Errorf("duplicate route name %q", rt.Name)
		}
		routes[rt.Name] = true
	}

	// validate entrypoints have a listen address, and TLS has both cert + key if present
	for _, ep := range cfg.Entrypoints {
		if ep.Listen == "" {
			return fmt.Errorf("entrypoint %q: missing listen address", ep.Name)
		}
		if ep.TLS != nil {
			if ep.TLS.CertPath == "" {
				return fmt.Errorf("entrypoint %q: tls missing cert path", ep.Name)
			}
			if ep.TLS.KeyPath == "" {
				return fmt.Errorf("entrypoint %q: tls missing key path", ep.Name)
			}
		}
	}

	// validate services have servers, known balance strategy, and positive weights
	for _, svc := range cfg.Services {
		if len(svc.Servers) == 0 {
			return fmt.Errorf("service %q: must have at least 1 server", svc.Name)
		}
		if svc.Balance != "" && !validStrategies[svc.Balance] {
			return fmt.Errorf("service %q: unknown balance strategy %q", svc.Name, svc.Balance)
		}
		for _, srv := range svc.Servers {
			if srv.Weight < 0 {
				return fmt.Errorf("service %q: server %q has negative weight", svc.Name, srv.URL)
			}
		}
	}

	// validate routes have a service + entrypoints, and all references point to existing names
	for _, rt := range cfg.Routes {
		if rt.Service == "" {
			return fmt.Errorf("route %q: missing service", rt.Name)
		}
		if !services[rt.Service] {
			return fmt.Errorf("route %q: references unknown service %q", rt.Name, rt.Service)
		}
		if len(rt.Entrypoints) == 0 {
			return fmt.Errorf("route %q: must have at least 1 entrypoint", rt.Name)
		}
		for _, ep := range rt.Entrypoints {
			if !entrypoints[ep] {
				return fmt.Errorf("route %q: references unknown entrypoint %q", rt.Name, ep)
			}
		}
		for _, mw := range rt.Middlewares {
			if !middlewares[mw] {
				return fmt.Errorf("route %q: references unknown middleware %q", rt.Name, mw)
			}
		}
	}

	return nil
}
