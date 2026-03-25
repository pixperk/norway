package stats

import (
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/pixperk/norway/balance"
)

func makeBackends(t *testing.T, urls ...string) []*balance.Backend {
	t.Helper()
	var backends []*balance.Backend
	for _, u := range urls {
		b, err := balance.NewBackend(u, 1)
		if err != nil {
			t.Fatalf("bad url %q: %v", u, err)
		}
		backends = append(backends, b)
	}
	return backends
}

func TestNewCollector(t *testing.T) {
	backends := makeBackends(t, "http://localhost:8001", "http://localhost:8002")
	c := NewCollector(backends)

	if c.TotalRequests.Load() != 0 {
		t.Error("total requests should start at 0")
	}
	if c.ActiveConns.Load() != 0 {
		t.Error("active conns should start at 0")
	}
	if time.Since(c.StartedAt) > time.Second {
		t.Error("started_at should be recent")
	}
}

func TestRecordRequest(t *testing.T) {
	c := NewCollector(nil)

	done := c.RecordRequest("api")

	if c.TotalRequests.Load() != 1 {
		t.Errorf("total requests = %d, want 1", c.TotalRequests.Load())
	}
	if c.ActiveConns.Load() != 1 {
		t.Errorf("active conns = %d, want 1", c.ActiveConns.Load())
	}

	// route stats should exist
	val, ok := c.Routes.Load("api")
	if !ok {
		t.Fatal("route 'api' not found in stats")
	}
	rs := val.(*RouteStats)
	if rs.Requests.Load() != 1 {
		t.Errorf("route requests = %d, want 1", rs.Requests.Load())
	}

	// simulate some work
	time.Sleep(1 * time.Millisecond)
	done()

	if c.ActiveConns.Load() != 0 {
		t.Errorf("active conns after done = %d, want 0", c.ActiveConns.Load())
	}
	if rs.TotalLatUs.Load() <= 0 {
		t.Error("latency should be recorded after done()")
	}
}

func TestRecordMultipleRoutes(t *testing.T) {
	c := NewCollector(nil)

	for range 5 {
		c.RecordRequest("api")()
	}
	for range 3 {
		c.RecordRequest("app")()
	}

	if c.TotalRequests.Load() != 8 {
		t.Errorf("total requests = %d, want 8", c.TotalRequests.Load())
	}

	val, _ := c.Routes.Load("api")
	if val.(*RouteStats).Requests.Load() != 5 {
		t.Errorf("api requests = %d, want 5", val.(*RouteStats).Requests.Load())
	}

	val, _ = c.Routes.Load("app")
	if val.(*RouteStats).Requests.Load() != 3 {
		t.Errorf("app requests = %d, want 3", val.(*RouteStats).Requests.Load())
	}
}

func TestActiveConnsTracking(t *testing.T) {
	c := NewCollector(nil)

	done1 := c.RecordRequest("api")
	done2 := c.RecordRequest("api")
	done3 := c.RecordRequest("app")

	if c.ActiveConns.Load() != 3 {
		t.Errorf("active conns = %d, want 3", c.ActiveConns.Load())
	}

	done1()
	if c.ActiveConns.Load() != 2 {
		t.Errorf("active conns after 1 done = %d, want 2", c.ActiveConns.Load())
	}

	done2()
	done3()
	if c.ActiveConns.Load() != 0 {
		t.Errorf("active conns after all done = %d, want 0", c.ActiveConns.Load())
	}
}

func TestHandlerJSON(t *testing.T) {
	backends := makeBackends(t, "http://localhost:8001", "http://localhost:8002")
	// mark second backend unhealthy
	backends[1].Healthy.Store(false)
	backends[0].ActiveConns.Store(5)

	c := NewCollector(backends)

	// record some requests
	c.RecordRequest("api")()
	c.RecordRequest("api")()
	c.RecordRequest("app")()

	req := httptest.NewRequest("GET", "/norway/stats", nil)
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("content-type = %q, want application/json", rec.Header().Get("Content-Type"))
	}

	var s snapshot
	if err := json.NewDecoder(rec.Body).Decode(&s); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if s.TotalRequests != 3 {
		t.Errorf("total_requests = %d, want 3", s.TotalRequests)
	}
	if s.ActiveConns != 0 {
		t.Errorf("active_conns = %d, want 0", s.ActiveConns)
	}
	if s.Uptime == "" {
		t.Error("uptime should not be empty")
	}

	// route stats
	if len(s.Routes) != 2 {
		t.Fatalf("routes count = %d, want 2", len(s.Routes))
	}
	if s.Routes["api"].Requests != 2 {
		t.Errorf("api requests = %d, want 2", s.Routes["api"].Requests)
	}
	if s.Routes["app"].Requests != 1 {
		t.Errorf("app requests = %d, want 1", s.Routes["app"].Requests)
	}

	// backend stats
	if len(s.Backends) != 2 {
		t.Fatalf("backends count = %d, want 2", len(s.Backends))
	}
	if !s.Backends[0].Healthy {
		t.Error("backend 0 should be healthy")
	}
	if s.Backends[1].Healthy {
		t.Error("backend 1 should be unhealthy")
	}
	if s.Backends[0].ActiveConns != 5 {
		t.Errorf("backend 0 active conns = %d, want 5", s.Backends[0].ActiveConns)
	}
}

func TestHandlerNoBackends(t *testing.T) {
	c := NewCollector(nil)
	c.RecordRequest("api")()

	req := httptest.NewRequest("GET", "/norway/stats", nil)
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, req)

	var s snapshot
	json.NewDecoder(rec.Body).Decode(&s)

	if s.TotalRequests != 1 {
		t.Errorf("total_requests = %d, want 1", s.TotalRequests)
	}
	if len(s.Backends) != 0 {
		t.Errorf("backends = %d, want 0", len(s.Backends))
	}
}

func TestConcurrentRecords(t *testing.T) {
	c := NewCollector(nil)
	var wg sync.WaitGroup

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			done := c.RecordRequest("api")
			time.Sleep(100 * time.Microsecond)
			done()
		}()
	}

	wg.Wait()

	if c.TotalRequests.Load() != 100 {
		t.Errorf("total requests = %d, want 100", c.TotalRequests.Load())
	}
	if c.ActiveConns.Load() != 0 {
		t.Errorf("active conns = %d, want 0", c.ActiveConns.Load())
	}

	val, _ := c.Routes.Load("api")
	rs := val.(*RouteStats)
	if rs.Requests.Load() != 100 {
		t.Errorf("route requests = %d, want 100", rs.Requests.Load())
	}
	if rs.TotalLatUs.Load() <= 0 {
		t.Error("total latency should be > 0")
	}
}

func TestAvgLatency(t *testing.T) {
	c := NewCollector(nil)

	// record a request with known sleep
	done := c.RecordRequest("slow")
	time.Sleep(5 * time.Millisecond)
	done()

	req := httptest.NewRequest("GET", "/norway/stats", nil)
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, req)

	var s snapshot
	json.NewDecoder(rec.Body).Decode(&s)

	// avg latency should be at least 4ms (some slack for timing)
	if s.Routes["slow"].AvgLatMs < 4.0 {
		t.Errorf("avg latency = %.2fms, expected >= 4ms", s.Routes["slow"].AvgLatMs)
	}
}
