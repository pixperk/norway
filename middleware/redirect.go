package middleware

import (
	"net/http"
	"strings"
)

// HTTPSRedirect returns a middleware that 301s any plain HTTP request
// to its HTTPS equivalent on the given target host (e.g. "example.com:8443").
// If targetHost is empty, the original request host is reused.
func HTTPSRedirect(targetHost string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// already HTTPS, nothing to do
			if r.TLS != nil {
				next.ServeHTTP(w, r)
				return
			}

			host := targetHost
			if host == "" {
				// strip any :port from the incoming host
				host = r.Host
				if i := strings.Index(host, ":"); i >= 0 {
					host = host[:i]
				}
			}

			target := "https://" + host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		})
	}
}
