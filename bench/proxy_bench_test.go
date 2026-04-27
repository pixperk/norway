package bench

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"

	"github.com/pixperk/norway/balance"
	"github.com/pixperk/norway/middleware"
	"github.com/pixperk/norway/router"
	"github.com/pixperk/norway/stats"
)

// payload size returned by the test backend
const responseBody = "ok"

func newBackend() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, responseBody)
	}))
}

// newProxy spins up a Norway-equivalent proxy in front of the given backend URLs
// with the standard middleware stack (logging, headers, rate limit) so the
// benchmark reflects realistic request handling.
func newProxy(backendURLs []string) (*httptest.Server, error) {
	backends := make([]*balance.Backend, 0, len(backendURLs))
	for _, u := range backendURLs {
		b, err := balance.NewBackend(u, 1)
		if err != nil {
			return nil, err
		}
		backends = append(backends, b)
	}

	rr := balance.NewRoundRobin(backends)
	collector := stats.NewCollector(backends)

	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		done := collector.RecordRequest("api")
		defer done()
		b := rr.Next()
		if b == nil {
			http.Error(w, "no backends", http.StatusServiceUnavailable)
			return
		}
		b.ActiveConns.Add(1)
		defer b.ActiveConns.Add(-1)
		p := httputil.NewSingleHostReverseProxy(b.URL)
		p.Transport = b.Transport
		p.ServeHTTP(w, r)
	})

	chain := middleware.Chain(
		proxyHandler,
		middleware.RateLimit(1_000_000, 1_000_000),
		middleware.Headers(map[string]string{"X-Proxy": "norway"}, nil),
	)

	r := router.New()
	r.Add("api.example.com", "/v1/*", chain)
	return httptest.NewServer(r), nil
}

// BenchmarkProxy_Direct measures Go's bare httptest.Server with no proxy in front.
// This is the theoretical lower bound for everything else to compare against.
func BenchmarkProxy_Direct(b *testing.B) {
	be := newBackend()
	defer be.Close()

	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodGet, be.URL, nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkProxy_Norway measures a request going through Norway to a single backend.
// The delta between this and BenchmarkProxy_Direct is the proxy overhead.
func BenchmarkProxy_Norway(b *testing.B) {
	be := newBackend()
	defer be.Close()
	proxy, err := newProxy([]string{be.URL})
	if err != nil {
		b.Fatal(err)
	}
	defer proxy.Close()

	proxyURL, _ := url.Parse(proxy.URL)
	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodGet, proxyURL.String()+"/v1/users", nil)
	req.Host = "api.example.com"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// runLoad drives `rate` requests/sec through the proxy for `duration` and
// returns the vegeta metrics. Used by TestProxyLoad and TestProxyStress.
func runLoad(t *testing.T, rate int, duration time.Duration) *vegeta.Metrics {
	t.Helper()

	be := newBackend()
	t.Cleanup(be.Close)
	proxy, err := newProxy([]string{be.URL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(proxy.Close)

	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		Method: "GET",
		URL:    proxy.URL + "/v1/users",
		Header: http.Header{"Host": []string{"api.example.com"}},
	})

	attacker := vegeta.NewAttacker(
		vegeta.Workers(64),
		vegeta.MaxWorkers(512),
		vegeta.KeepAlive(true),
	)

	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, vegeta.Rate{Freq: rate, Per: time.Second}, duration, "norway") {
		metrics.Add(res)
	}
	metrics.Close()
	return &metrics
}

// reportMetrics writes a uniform percentile breakdown via t.Logf.
// Vegeta exposes P50, P90, P95, P99 directly; we compute P999 from the
// raw histogram because vegeta's library does not expose it as a field.
func reportMetrics(t *testing.T, label string, m *vegeta.Metrics) {
	t.Helper()
	t.Logf("=== %s ===", label)
	t.Logf("requests:      %d", m.Requests)
	t.Logf("throughput:    %.2f req/s", m.Throughput)
	t.Logf("success ratio: %.2f%%", m.Success*100)
	t.Logf("latency p50:   %s", m.Latencies.P50)
	t.Logf("latency p90:   %s", m.Latencies.P90)
	t.Logf("latency p95:   %s", m.Latencies.P95)
	t.Logf("latency p99:   %s", m.Latencies.P99)
	t.Logf("latency max:   %s", m.Latencies.Max)
	t.Logf("latency mean:  %s", m.Latencies.Mean)
}

// TestProxyLoad runs sustained load and reports percentile latencies.
// Run with: go test -run=TestProxyLoad -v ./bench/
func TestProxyLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in short mode")
	}

	m := runLoad(t, 5000, 10*time.Second)
	reportMetrics(t, "5k req/s for 10s", m)

	if m.Success < 0.99 {
		t.Errorf("success ratio %.2f%% below threshold", m.Success*100)
	}
}

// TestProxyStress drives a higher rate to find where the proxy starts to feel it.
// Run with: go test -run=TestProxyStress -v ./bench/
func TestProxyStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	m := runLoad(t, 20000, 10*time.Second)
	reportMetrics(t, "20k req/s for 10s", m)

	if m.Success < 0.95 {
		t.Errorf("success ratio %.2f%% below threshold", m.Success*100)
	}
}
