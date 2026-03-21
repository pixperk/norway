package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterHostDispatch(t *testing.T) {
	r := New()

	r.Add("api.example.com", "/v1/users", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("api-users"))
	}))
	r.Add("app.example.com", "/dashboard", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("app-dashboard"))
	}))

	tests := []struct {
		host string
		path string
		want string
		code int
	}{
		{"api.example.com", "/v1/users", "api-users", 200},
		{"app.example.com", "/dashboard", "app-dashboard", 200},
		{"api.example.com", "/nope", "", 404},
		{"unknown.com", "/v1/users", "", 404},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		req.Host = tt.host
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != tt.code {
			t.Errorf("host=%q path=%q: status = %d, want %d", tt.host, tt.path, w.Code, tt.code)
		}
		if tt.want != "" && w.Body.String() != tt.want {
			t.Errorf("host=%q path=%q: body = %q, want %q", tt.host, tt.path, w.Body.String(), tt.want)
		}
	}
}

func TestRouterStripPort(t *testing.T) {
	r := New()
	r.Add("api.example.com", "/ping", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("pong"))
	}))

	req := httptest.NewRequest("GET", "/ping", nil)
	req.Host = "api.example.com:8080"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "pong" {
		t.Errorf("expected 'pong', got %q", w.Body.String())
	}
}

func TestRouterParams(t *testing.T) {
	r := New()
	r.Add("api.example.com", "/users/:id", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		params := Params(req)
		w.Write([]byte("user:" + params["id"]))
	}))

	req := httptest.NewRequest("GET", "/users/42", nil)
	req.Host = "api.example.com"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "user:42" {
		t.Errorf("expected 'user:42', got %q", w.Body.String())
	}
}
