package balance

import (
	"testing"
)

func makeBackends(n int) []*Backend {
	backends := make([]*Backend, n)
	for i := range n {
		b, _ := NewBackend("http://localhost:"+string(rune('0'+i)), 1)
		backends[i] = b
	}
	return backends
}

func TestRoundRobinDistribution(t *testing.T) {
	backends := makeBackends(3)
	rr := NewRoundRobin(backends)

	// 9 requests should hit each backend 3 times
	counts := map[*Backend]int{}
	for range 9 {
		b := rr.Next()
		counts[b]++
	}

	for _, b := range backends {
		if counts[b] != 3 {
			t.Errorf("expected 3 hits, got %d", counts[b])
		}
	}
}

func TestRoundRobinSkipsUnhealthy(t *testing.T) {
	backends := makeBackends(3)
	rr := NewRoundRobin(backends)

	// mark backend 1 as unhealthy
	backends[1].Healthy.Store(false)

	for range 6 {
		b := rr.Next()
		if b == backends[1] {
			t.Error("unhealthy backend should be skipped")
		}
	}
}

func TestRoundRobinAllDown(t *testing.T) {
	backends := makeBackends(2)
	rr := NewRoundRobin(backends)

	backends[0].Healthy.Store(false)
	backends[1].Healthy.Store(false)

	if b := rr.Next(); b != nil {
		t.Errorf("expected nil when all backends are down, got %v", b.URL)
	}
}

func TestWeightedDistribution(t *testing.T) {
	b1, _ := NewBackend("http://localhost:8001", 3)
	b2, _ := NewBackend("http://localhost:8002", 1)
	w := NewWeighted([]*Backend{b1, b2})

	counts := map[*Backend]int{}
	for range 8 {
		b := w.Next()
		counts[b]++
	}

	// b1 should get 3x the traffic of b2
	// 8 requests over [b1,b1,b1,b2] = b1 gets 6, b2 gets 2
	if counts[b1] != 6 {
		t.Errorf("b1 expected 6 hits, got %d", counts[b1])
	}
	if counts[b2] != 2 {
		t.Errorf("b2 expected 2 hits, got %d", counts[b2])
	}
}

func TestWeightedSkipsUnhealthy(t *testing.T) {
	b1, _ := NewBackend("http://localhost:8001", 3)
	b2, _ := NewBackend("http://localhost:8002", 1)
	w := NewWeighted([]*Backend{b1, b2})

	b1.Healthy.Store(false)

	for range 4 {
		b := w.Next()
		if b != b2 {
			t.Error("should only hit b2 when b1 is down")
		}
	}
}

func TestLeastConnPicksLowest(t *testing.T) {
	backends := makeBackends(3)
	lc := NewLeastConn(backends)

	// simulate active connections
	backends[0].ActiveConns.Store(5)
	backends[1].ActiveConns.Store(2)
	backends[2].ActiveConns.Store(8)

	b := lc.Next()
	if b != backends[1] {
		t.Errorf("expected backend with 2 conns, got one with %d", b.ActiveConns.Load())
	}
}

func TestLeastConnSkipsUnhealthy(t *testing.T) {
	backends := makeBackends(3)
	lc := NewLeastConn(backends)

	// backend 1 has lowest conns but is unhealthy
	backends[0].ActiveConns.Store(5)
	backends[1].ActiveConns.Store(1)
	backends[1].Healthy.Store(false)
	backends[2].ActiveConns.Store(3)

	b := lc.Next()
	if b != backends[2] {
		t.Errorf("expected backend[2] with 3 conns, got one with %d", b.ActiveConns.Load())
	}
}

func TestLeastConnAllDown(t *testing.T) {
	backends := makeBackends(2)
	lc := NewLeastConn(backends)

	backends[0].Healthy.Store(false)
	backends[1].Healthy.Store(false)

	if b := lc.Next(); b != nil {
		t.Errorf("expected nil when all backends are down, got %v", b.URL)
	}
}

func TestLeastConnIncDec(t *testing.T) {
	backends := makeBackends(2)
	lc := NewLeastConn(backends)

	// simulate request flow: pick, increment, pick again
	b1 := lc.Next()
	b1.ActiveConns.Add(1)

	b2 := lc.Next()
	// should pick the other one since b1 now has 1 active conn
	if b2 == b1 {
		t.Error("should pick the other backend after incrementing")
	}

	// decrement b1, now both are equal, next should pick b1 (first in list with 0 conns)
	b1.ActiveConns.Add(-1)
	b3 := lc.Next()
	if b3 != backends[0] {
		t.Error("expected first backend when both have equal conns")
	}
}

func TestEmptyBackends(t *testing.T) {
	rr := NewRoundRobin(nil)
	if b := rr.Next(); b != nil {
		t.Error("round-robin with nil backends should return nil")
	}

	w := NewWeighted(nil)
	if b := w.Next(); b != nil {
		t.Error("weighted with nil backends should return nil")
	}

	lc := NewLeastConn(nil)
	if b := lc.Next(); b != nil {
		t.Error("least-conn with nil backends should return nil")
	}
}
