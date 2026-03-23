package health

import (
	"log"
	"net/http"
	"time"

	"github.com/pixperk/norway/balance"
)

// Checker periodically pings backends and marks them healthy/unhealthy.
// One checker per service. Runs in a background goroutine.
type Checker struct {
	backends []*balance.Backend
	path     string
	interval time.Duration
	timeout  time.Duration
	client   *http.Client
	stop     chan struct{}
}

func New(backends []*balance.Backend, path string, interval, timeout time.Duration) *Checker {
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	if interval == 0 {
		interval = 10 * time.Second
	}
	return &Checker{
		backends: backends,
		path:     path,
		interval: interval,
		timeout:  timeout,
		client:   &http.Client{Timeout: timeout},
		stop:     make(chan struct{}),
	}
}

// Start begins the health check loop in a background goroutine
func (c *Checker) Start() {
	go func() {
		// check immediately on startup
		c.checkAll()

		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.checkAll()
			case <-c.stop:
				return
			}
		}
	}()
}

// Stop signals the health checker to stop
func (c *Checker) Stop() {
	close(c.stop)
}

func (c *Checker) checkAll() {
	for _, b := range c.backends {
		healthy := c.ping(b)
		was := b.Healthy.Load()

		b.Healthy.Store(healthy)

		// only log on state change
		if was && !healthy {
			log.Printf("health: %s is DOWN", b.URL)
		}
		if !was && healthy {
			log.Printf("health: %s is UP", b.URL)
		}
	}
}

// ping sends a GET to the backend's health path, returns true if 2xx
func (c *Checker) ping(b *balance.Backend) bool {
	url := b.URL.String() + c.path
	resp, err := c.client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
