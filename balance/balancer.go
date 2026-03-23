package balance

import (
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

// Balancer picks the next healthy backend from a pool
type Balancer interface {
	Next() *Backend
	All() []*Backend
}

// Backend represents a single upstream server.
// Healthy and ActiveConns are atomic because the health checker
// and request handlers access them from different goroutines.
type Backend struct {
	URL         *url.URL
	Weight      int
	Healthy     atomic.Bool
	ActiveConns atomic.Int64
	Transport   *http.Transport
}

func NewBackend(rawURL string, weight int) (*Backend, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if weight <= 0 {
		weight = 1
	}
	b := &Backend{
		URL:    u,
		Weight: weight,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 32,
			MaxConnsPerHost:     64,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	b.Healthy.Store(true)
	return b, nil
}
