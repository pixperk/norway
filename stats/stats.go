package stats

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pixperk/norway/balance"
)

// Collector holds live counters for the entire proxy.
// All fields are atomic so middlewares and handlers can write without locks.
type Collector struct {
	TotalRequests atomic.Int64
	ActiveConns   atomic.Int64
	StartedAt     time.Time
	Routes        sync.Map // route name -> *RouteStats
	backends      []*balance.Backend
}

// RouteStats tracks per-route request count and total latency (for computing avg)
type RouteStats struct {
	Requests   atomic.Int64
	TotalLatUs atomic.Int64 // microseconds, divide by Requests for avg
}

func NewCollector(backends []*balance.Backend) *Collector {
	return &Collector{
		StartedAt: time.Now(),
		backends:  backends,
	}
}

// RecordRequest increments total requests and active connections.
// Returns a done func that decrements active conns and records latency for the route.
func (c *Collector) RecordRequest(routeName string) func() {
	c.TotalRequests.Add(1)
	c.ActiveConns.Add(1)
	start := time.Now()

	// get or create route stats
	val, _ := c.Routes.LoadOrStore(routeName, &RouteStats{})
	rs := val.(*RouteStats)
	rs.Requests.Add(1)

	return func() {
		c.ActiveConns.Add(-1)
		latUs := time.Since(start).Microseconds()
		rs.TotalLatUs.Add(latUs)
	}
}

// json output types

type snapshot struct {
	Uptime        string            `json:"uptime"`
	TotalRequests int64             `json:"total_requests"`
	ActiveConns   int64             `json:"active_conns"`
	Routes        map[string]routeSnap `json:"routes"`
	Backends      []backendSnap     `json:"backends"`
}

type routeSnap struct {
	Requests   int64   `json:"requests"`
	AvgLatMs   float64 `json:"avg_latency_ms"`
}

type backendSnap struct {
	URL         string `json:"url"`
	Healthy     bool   `json:"healthy"`
	ActiveConns int64  `json:"active_conns"`
}

// Handler returns an http.Handler that serves /norway/stats as JSON
func (c *Collector) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := snapshot{
			Uptime:        time.Since(c.StartedAt).Round(time.Second).String(),
			TotalRequests: c.TotalRequests.Load(),
			ActiveConns:   c.ActiveConns.Load(),
			Routes:        make(map[string]routeSnap),
		}

		// collect route stats
		c.Routes.Range(func(key, val any) bool {
			name := key.(string)
			rs := val.(*RouteStats)
			reqs := rs.Requests.Load()
			avgMs := 0.0
			if reqs > 0 {
				avgMs = float64(rs.TotalLatUs.Load()) / float64(reqs) / 1000.0
			}
			s.Routes[name] = routeSnap{
				Requests: reqs,
				AvgLatMs: avgMs,
			}
			return true
		})

		// collect backend stats (live pointers, always current)
		for _, b := range c.backends {
			s.Backends = append(s.Backends, backendSnap{
				URL:         b.URL.String(),
				Healthy:     b.Healthy.Load(),
				ActiveConns: b.ActiveConns.Load(),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)
	})
}
