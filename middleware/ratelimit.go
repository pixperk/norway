package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// bucket is a token bucket for a single client.
// Tokens refill at a fixed rate. Each request costs 1 token.
// If the bucket is empty, the request is rejected.
type bucket struct {
	tokens   float64
	capacity float64
	rate     float64 // tokens per second
	last     time.Time
	mu       sync.Mutex
}

// allow checks if a request can proceed.
// It refills tokens based on elapsed time, then tries to consume one.
func (b *bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.tokens = math.Min(b.capacity, b.tokens+elapsed*b.rate)
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// retryAfter returns how many seconds until a token is available
func (b *bucket) retryAfter() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.rate <= 0 {
		return 60
	}
	wait := (1 - b.tokens) / b.rate
	return int(math.Ceil(wait))
}

// RateLimit returns a middleware that limits requests per client IP
// using the token bucket algorithm.
//
// rate: tokens added per second (e.g. 100 means 100 req/s sustained)
// burst: bucket capacity (e.g. 50 means up to 50 requests can fire at once)
func RateLimit(rate float64, burst int) Middleware {
	buckets := sync.Map{} // client IP -> *bucket

	// periodically clean up stale buckets to avoid memory leaks
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			now := time.Now()
			buckets.Range(func(key, val any) bool {
				b := val.(*bucket)
				b.mu.Lock()
				idle := now.Sub(b.last)
				b.mu.Unlock()
				// remove buckets idle for more than 10 minutes
				if idle > 10*time.Minute {
					buckets.Delete(key)
				}
				return true
			})
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// extract client IP without port
			ip := r.RemoteAddr
			if i := strings.LastIndex(ip, ":"); i != -1 {
				ip = ip[:i]
			}

			// get or create bucket for this IP
			val, _ := buckets.LoadOrStore(ip, &bucket{
				tokens:   float64(burst),
				capacity: float64(burst),
				rate:     rate,
				last:     time.Now(),
			})
			b := val.(*bucket)

			if !b.allow() {
				retry := b.retryAfter()
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retry))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
