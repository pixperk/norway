package balance

import (
	"fmt"
	"testing"
)

// benchBackends creates n healthy backends with the given weight.
func benchBackends(n, weight int) []*Backend {
	bs := make([]*Backend, n)
	for i := 0; i < n; i++ {
		b, _ := NewBackend(fmt.Sprintf("http://localhost:%d", 8000+i), weight)
		bs[i] = b
	}
	return bs
}

func BenchmarkRoundRobin_Next(b *testing.B) {
	rr := NewRoundRobin(benchBackends(8, 1))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = rr.Next()
	}
}

func BenchmarkWeighted_Next(b *testing.B) {
	bs := benchBackends(4, 1)
	bs[0].Weight = 5
	bs[1].Weight = 3
	bs[2].Weight = 1
	bs[3].Weight = 1
	w := NewWeighted(bs)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = w.Next()
	}
}

func BenchmarkLeastConn_Next_2(b *testing.B)  { benchLeastConn(b, 2) }
func BenchmarkLeastConn_Next_8(b *testing.B)  { benchLeastConn(b, 8) }
func BenchmarkLeastConn_Next_32(b *testing.B) { benchLeastConn(b, 32) }

func benchLeastConn(b *testing.B, n int) {
	lc := NewLeastConn(benchBackends(n, 1))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = lc.Next()
	}
}

// Parallel variants measure contention behavior. RoundRobin uses an atomic
// counter so contention should be minimal. Weighted uses a lock-free pre-expanded
// array. LeastConn scans atomic counters with no shared mutex.

func BenchmarkRoundRobin_Next_Parallel(b *testing.B) {
	rr := NewRoundRobin(benchBackends(8, 1))
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = rr.Next()
		}
	})
}

func BenchmarkWeighted_Next_Parallel(b *testing.B) {
	bs := benchBackends(4, 1)
	bs[0].Weight = 5
	bs[1].Weight = 3
	w := NewWeighted(bs)
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = w.Next()
		}
	})
}

func BenchmarkLeastConn_Next_Parallel(b *testing.B) {
	lc := NewLeastConn(benchBackends(8, 1))
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = lc.Next()
		}
	})
}
