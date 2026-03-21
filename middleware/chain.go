package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	//wrap in reverse order since the first middleware should be the outermost
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	//if i = 3, handler = m3(handler) -> m2(m3(handler)) -> m1(m2(m3(handler)))
	return handler
}
