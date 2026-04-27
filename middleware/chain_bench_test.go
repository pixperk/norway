package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var benchHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// passthrough is a no-op middleware used to measure pure chain overhead
// without any work being done inside each layer.
func passthrough(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func runChainBench(b *testing.B, h http.Handler) {
	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h.ServeHTTP(w, req)
	}
}

func BenchmarkChain_NoMiddleware(b *testing.B) {
	runChainBench(b, benchHandler)
}

func BenchmarkChain_OneMiddleware(b *testing.B) {
	h := Chain(benchHandler, passthrough)
	runChainBench(b, h)
}

func BenchmarkChain_FiveMiddlewares(b *testing.B) {
	h := Chain(benchHandler, passthrough, passthrough, passthrough, passthrough, passthrough)
	runChainBench(b, h)
}

func BenchmarkHeaders(b *testing.B) {
	h := Chain(benchHandler, Headers(map[string]string{
		"X-Proxy":   "norway",
		"X-Version": "0.1",
	}, []string{"Server"}))
	runChainBench(b, h)
}

func BenchmarkRateLimit_Allow(b *testing.B) {
	// rate=1e6 / burst=1e6 means we will essentially never be denied
	h := Chain(benchHandler, RateLimit(1_000_000, 1_000_000))
	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h.ServeHTTP(w, req)
	}
}

// BenchmarkRateLimit_Parallel exercises the per-IP token bucket sync.Map under
// real concurrent contention. Each goroutine uses its own RemoteAddr so we
// measure the hot path across distinct buckets, not contention on one bucket.
func BenchmarkRateLimit_Parallel(b *testing.B) {
	h := Chain(benchHandler, RateLimit(1_000_000, 1_000_000))
	b.ResetTimer()
	b.ReportAllocs()

	var counter int
	b.RunParallel(func(pb *testing.PB) {
		counter++
		req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
		req.RemoteAddr = "10.0.0." + string(rune('0'+counter%10)) + ":1234"
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, req)
		}
	})
}

// BenchmarkLogging measures the JSON serialization cost of the logging middleware.
// We redirect os.Stdout to /dev/null because the middleware writes there directly,
// and the bench output would otherwise be buried in log lines.
func BenchmarkLogging(b *testing.B) {
	// silence the JSON output during the bench
	devnull, _ := os.Open(os.DevNull)
	if devnull == nil {
		devnull, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	}
	origStdout := os.Stdout
	os.Stdout = devnull
	defer func() {
		os.Stdout = origStdout
		_ = io.Discard
	}()

	h := Chain(benchHandler, Logging())
	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	req.Host = "api.example.com"
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("User-Agent", "norway-bench/1.0")
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h.ServeHTTP(w, req)
	}
}

// BenchmarkHTTPSRedirect measures the cost of issuing a 301 redirect.
func BenchmarkHTTPSRedirect(b *testing.B) {
	h := HTTPSRedirect("api.example.com:8443")(benchHandler)
	req := httptest.NewRequest(http.MethodGet, "/v1/users?key=val", nil)
	req.Host = "api.example.com"
	w := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h.ServeHTTP(w, req)
	}
}
