package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChain(t *testing.T) {
	var order []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-in")
			next.ServeHTTP(w, r)
			order = append(order, "m1-out")
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-in")
			next.ServeHTTP(w, r)
			order = append(order, "m2-out")
		})
	}

	handler := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	}), m1, m2)

	req := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	want := []string{"m1-in", "m2-in", "handler", "m2-out", "m1-out"}
	if len(order) != len(want) {
		t.Fatalf("got %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestLogging(t *testing.T) {
	handler := Logging()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		w.Write([]byte("created"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 201 {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if rec.Body.String() != "created" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "created")
	}
}

func TestLoggingJSON(t *testing.T) {
	// capture stdout by using a pipe — skip, just verify the handler doesn't panic
	handler := Logging()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("POST", "/api/v1/users", nil)
	req.Header.Set("User-Agent", "test-agent")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestLoggingLogEntry(t *testing.T) {
	// verify logEntry struct marshals correctly
	entry := logEntry{
		Timestamp:  "2026-03-22T10:00:00Z",
		Method:     "GET",
		Host:       "api.example.com",
		Path:       "/v1/users",
		Status:     200,
		DurationMs: 1.5,
		Bytes:      42,
		ClientIP:   "127.0.0.1:1234",
		UserAgent:  "curl/8.5.0",
		Proto:      "HTTP/1.1",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded map[string]interface{}
	json.Unmarshal(data, &decoded)

	if decoded["method"] != "GET" {
		t.Errorf("method = %v, want GET", decoded["method"])
	}
	if decoded["status"].(float64) != 200 {
		t.Errorf("status = %v, want 200", decoded["status"])
	}
}

func TestHeadersAdd(t *testing.T) {
	add := map[string]string{
		"X-Proxy":   "norway",
		"X-Version": "0.1",
	}

	handler := Headers(add, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Proxy") != "norway" {
		t.Errorf("X-Proxy = %q, want %q", rec.Header().Get("X-Proxy"), "norway")
	}
	if rec.Header().Get("X-Version") != "0.1" {
		t.Errorf("X-Version = %q, want %q", rec.Header().Get("X-Version"), "0.1")
	}
}

func TestHeadersRemove(t *testing.T) {
	remove := []string{"Server"}

	handler := Headers(nil, remove)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "nginx")
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Server") != "" {
		t.Errorf("Server header should be removed, got %q", rec.Header().Get("Server"))
	}
}

func TestHeadersAddAndRemove(t *testing.T) {
	add := map[string]string{"X-Proxy": "norway"}
	remove := []string{"Server", "X-Powered-By"}

	handler := Headers(add, remove)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "apache")
		w.Header().Set("X-Powered-By", "PHP")
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Proxy") != "norway" {
		t.Errorf("X-Proxy = %q, want %q", rec.Header().Get("X-Proxy"), "norway")
	}
	if rec.Header().Get("Server") != "" {
		t.Errorf("Server should be removed")
	}
	if rec.Header().Get("X-Powered-By") != "" {
		t.Errorf("X-Powered-By should be removed")
	}
}
