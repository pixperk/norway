package router

import (
	"net/http"
	"testing"
)

// dummy handler that stores a name so we can identify which route matched
type namedHandler string

func (h namedHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestStaticRoutes(t *testing.T) {
	tree := NewTree()
	tree.Insert("/api/v1/users", namedHandler("users"))
	tree.Insert("/api/v1/orders", namedHandler("orders"))
	tree.Insert("/api/v2/users", namedHandler("users-v2"))
	tree.Insert("/health", namedHandler("health"))

	tests := []struct {
		path string
		want string
	}{
		{"/api/v1/users", "users"},
		{"/api/v1/orders", "orders"},
		{"/api/v2/users", "users-v2"},
		{"/health", "health"},
	}

	for _, tt := range tests {
		h, _ := tree.Lookup(tt.path)
		if h == nil {
			t.Errorf("Lookup(%q) = nil, want %q", tt.path, tt.want)
			continue
		}
		if got := string(h.(namedHandler)); got != tt.want {
			t.Errorf("Lookup(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestStaticNotFound(t *testing.T) {
	tree := NewTree()
	tree.Insert("/api/v1/users", namedHandler("users"))

	noMatch := []string{
		"/api/v1",
		"/api/v1/users/extra",
		"/api/v2",
		"/",
		"/nothing",
	}

	for _, path := range noMatch {
		h, _ := tree.Lookup(path)
		if h != nil {
			t.Errorf("Lookup(%q) should be nil, got %q", path, string(h.(namedHandler)))
		}
	}
}

func TestParamRoutes(t *testing.T) {
	tree := NewTree()
	tree.Insert("/users/:id", namedHandler("user-by-id"))
	tree.Insert("/users/:id/posts", namedHandler("user-posts"))
	tree.Insert("/posts/:postId/comments/:commentId", namedHandler("comment"))

	tests := []struct {
		path       string
		want       string
		wantParams map[string]string
	}{
		{"/users/42", "user-by-id", map[string]string{"id": "42"}},
		{"/users/abc", "user-by-id", map[string]string{"id": "abc"}},
		{"/users/42/posts", "user-posts", map[string]string{"id": "42"}},
		{"/posts/10/comments/5", "comment", map[string]string{"postId": "10", "commentId": "5"}},
	}

	for _, tt := range tests {
		h, params := tree.Lookup(tt.path)
		if h == nil {
			t.Errorf("Lookup(%q) = nil, want %q", tt.path, tt.want)
			continue
		}
		if got := string(h.(namedHandler)); got != tt.want {
			t.Errorf("Lookup(%q) = %q, want %q", tt.path, got, tt.want)
		}
		for k, v := range tt.wantParams {
			if params[k] != v {
				t.Errorf("Lookup(%q) param %q = %q, want %q", tt.path, k, params[k], v)
			}
		}
	}
}

func TestWildcardRoutes(t *testing.T) {
	tree := NewTree()
	tree.Insert("/static/*filepath", namedHandler("static"))
	tree.Insert("/api/v1/users", namedHandler("users"))

	tests := []struct {
		path       string
		want       string
		wantParams map[string]string
	}{
		{"/static/css/main.css", "static", map[string]string{"filepath": "css/main.css"}},
		{"/static/js/app.js", "static", map[string]string{"filepath": "js/app.js"}},
		{"/static/logo.png", "static", map[string]string{"filepath": "logo.png"}},
		{"/api/v1/users", "users", map[string]string{}},
	}

	for _, tt := range tests {
		h, params := tree.Lookup(tt.path)
		if h == nil {
			t.Errorf("Lookup(%q) = nil, want %q", tt.path, tt.want)
			continue
		}
		if got := string(h.(namedHandler)); got != tt.want {
			t.Errorf("Lookup(%q) = %q, want %q", tt.path, got, tt.want)
		}
		for k, v := range tt.wantParams {
			if params[k] != v {
				t.Errorf("Lookup(%q) param %q = %q, want %q", tt.path, k, params[k], v)
			}
		}
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/api/v1/users", []string{"/api/v1/users"}},
		{"/users/:id", []string{"/users/", ":id"}},
		{"/users/:id/posts", []string{"/users/", ":id", "/posts"}},
		{"/static/*filepath", []string{"/static/", "*filepath"}},
		{"/posts/:postId/comments/:commentId", []string{"/posts/", ":postId", "/comments/", ":commentId"}},
	}

	for _, tt := range tests {
		got := splitPath(tt.path)
		if len(got) != len(tt.want) {
			t.Errorf("splitPath(%q) = %v, want %v", tt.path, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitPath(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
			}
		}
	}
}

func TestOverlappingRoutes(t *testing.T) {
	tree := NewTree()
	tree.Insert("/api", namedHandler("api-root"))
	tree.Insert("/api/v1", namedHandler("api-v1"))
	tree.Insert("/api/v1/users", namedHandler("api-v1-users"))
	tree.Insert("/api/v2", namedHandler("api-v2"))

	tests := []struct {
		path string
		want string
	}{
		{"/api", "api-root"},
		{"/api/v1", "api-v1"},
		{"/api/v1/users", "api-v1-users"},
		{"/api/v2", "api-v2"},
	}

	for _, tt := range tests {
		h, _ := tree.Lookup(tt.path)
		if h == nil {
			t.Errorf("Lookup(%q) = nil, want %q", tt.path, tt.want)
			continue
		}
		if got := string(h.(namedHandler)); got != tt.want {
			t.Errorf("Lookup(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
