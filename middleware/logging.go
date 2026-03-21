package middleware

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

// responseRecorder wraps http.ResponseWriter to capture status code and bytes written.
// http.ResponseWriter doesn't expose these after the fact, so we intercept
// WriteHeader and Write calls to record them for logging.
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// logEntry is a single structured log line written as JSON to stdout
type logEntry struct {
	Timestamp  string  `json:"ts"`
	Method     string  `json:"method"`
	Host       string  `json:"host"`
	Path       string  `json:"path"`
	Status     int     `json:"status"`
	DurationMs float64 `json:"duration_ms"`
	Bytes      int     `json:"bytes"`
	ClientIP   string  `json:"client_ip"`
	UserAgent  string  `json:"user_agent"`
	Proto      string  `json:"proto"`
}

// Logging returns a middleware that emits one JSON log line per request.
// It records method, host, path, status, duration, response size, client IP,
// user agent, and protocol.
func Logging() Middleware {
	encoder := json.NewEncoder(os.Stdout)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			// default status to 200 in case handler never calls WriteHeader
			rec := &responseRecorder{ResponseWriter: w, status: 200}

			next.ServeHTTP(rec, r)

			// log after the request completes so we have status, bytes, and duration
			encoder.Encode(logEntry{
				Timestamp:  start.UTC().Format(time.RFC3339Nano),
				Method:     r.Method,
				Host:       r.Host,
				Path:       r.URL.Path,
				Status:     rec.status,
				DurationMs: float64(time.Since(start).Microseconds()) / 1000.0,
				Bytes:      rec.bytes,
				ClientIP:   r.RemoteAddr,
				UserAgent:  r.UserAgent(),
				Proto:      r.Proto,
			})
		})
	}
}
