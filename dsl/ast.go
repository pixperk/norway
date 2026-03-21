package dsl

type Config struct {
	Entrypoints []Entrypoint
	Services    []Service
	Middlewares []Middleware
	Routes      []Route
}

type Entrypoint struct {
	Name   string
	Listen string
	TLS    *TLSConfig // optional
}

type TLSConfig struct {
	CertPath string
	KeyPath  string
}

type Service struct {
	Name    string
	Balance string             //for load balancing e.g. "round-robin", "least-conn"
	Health  *HealthCheckConfig //optional
	Servers []Server
}

type HealthCheckConfig struct {
	Path     string
	Interval string // e.g. "10s"
	Timeout  string // e.g. "2s"
}

type Server struct {
	URL    string
	Weight int // optional, default 1
}

type Middleware struct {
	Name   string
	Type   string            // e.g. "ratelimit", "headers", "logging"
	Config map[string]string // type-specific config, e.g. rate/format
}

type Route struct {
	Name        string
	Entrypoints []string
	Host        string
	Path        string
	Service     string
	Middlewares []string // names of middlewares to use
}
