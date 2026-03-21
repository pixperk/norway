package router

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const paramsKey contextKey = "params"

// Router dispatches requests to the correct radix tree based on the Host header
type Router struct {
	trees map[string]*Tree // host -> radix tree
}

func New() *Router {
	return &Router{trees: make(map[string]*Tree)}
}

// Add registers a handler for a given host + path combination
func (r *Router) Add(host, path string, handler http.Handler) {
	tree, ok := r.trees[host]
	if !ok {
		tree = NewTree()
		r.trees[host] = tree
	}
	tree.Insert(path, handler)
}

// ServeHTTP matches the request to a route and serves it
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	host := req.Host
	// strip port from host if present
	if i := strings.IndexByte(host, ':'); i != -1 {
		host = host[:i]
	}

	tree, ok := r.trees[host]
	if !ok {
		http.NotFound(w, req)
		return
	}

	handler, params := tree.Lookup(req.URL.Path)
	if handler == nil {
		http.NotFound(w, req)
		return
	}

	// attach params to request context so handlers can access them
	ctx := context.WithValue(req.Context(), paramsKey, params)
	handler.ServeHTTP(w, req.WithContext(ctx))
}

// Params extracts route params from the request context
func Params(r *http.Request) map[string]string {
	p, _ := r.Context().Value(paramsKey).(map[string]string)
	return p
}
