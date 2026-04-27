package stats

import (
	"net/http/httptest"
	"testing"

	"github.com/pixperk/norway/balance"
)

func newCollectorWithBackends(n int) *Collector {
	bs := make([]*balance.Backend, n)
	for i := 0; i < n; i++ {
		b, _ := balance.NewBackend("http://localhost:8001", 1)
		bs[i] = b
	}
	return NewCollector(bs)
}

// BenchmarkRecordRequest measures the per-request stats overhead.
// This runs on the hot path inside balancedProxy, so it must be cheap.
func BenchmarkRecordRequest(b *testing.B) {
	c := newCollectorWithBackends(2)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		done := c.RecordRequest("api")
		done()
	}
}

// BenchmarkRecordRequest_Parallel exercises the sync.Map under contention.
// The first call to LoadOrStore is the expensive one; subsequent calls hit
// the read-only fast path.
func BenchmarkRecordRequest_Parallel(b *testing.B) {
	c := newCollectorWithBackends(2)
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			done := c.RecordRequest("api")
			done()
		}
	})
}

// BenchmarkStatsHandler measures snapshot serialization for /norway/stats.
// Includes routes Range, backend pointer reads, and JSON encoding.
func BenchmarkStatsHandler(b *testing.B) {
	c := newCollectorWithBackends(8)
	// pre-populate some routes so Range has work to do
	for _, name := range []string{"api", "app", "admin", "static"} {
		done := c.RecordRequest(name)
		done()
	}
	h := c.Handler()
	req := httptest.NewRequest("GET", "/norway/stats", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}
