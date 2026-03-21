package middleware

import "net/http"

// headersRecorder intercepts WriteHeader and Write calls from the proxy.
// We never call WriteHeader ourselves. The proxy (httputil.ReverseProxy) calls it
// when it gets a response from the backend. Since the proxy thinks this recorder
// is the real ResponseWriter, our WriteHeader runs first, we add/remove headers,
// then forward to the actual writer.
type headersRecorder struct {
	http.ResponseWriter
	wroteHeader bool
	add         map[string]string
	remove      []string
}

func (h *headersRecorder) WriteHeader(status int) {
	if h.wroteHeader {
		return
	}

	h.wroteHeader = true
	//add headers
	for k, v := range h.add {
		h.ResponseWriter.Header().Set(k, v)
	}

	//remove headers
	for _, k := range h.remove {
		h.ResponseWriter.Header().Del(k)
	}

	h.ResponseWriter.WriteHeader(status)
}

func Headers(add map[string]string, remove []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &headersRecorder{
				ResponseWriter: w,
				add:            add,
				remove:         remove,
			}
			next.ServeHTTP(rec, r)
		})
	}
}
