package router

import (
	"fmt"
	"net/http"
	"testing"
)

// noop handler used in benchmarks so we are not measuring the handler itself
var noopHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

// buildTree creates a tree with `n` static routes plus one param and one wildcard route.
// Used by lookup benchmarks to simulate a realistic routing table.
func buildTree(n int) *Tree {
	t := NewTree()
	for i := 0; i < n; i++ {
		t.Insert(fmt.Sprintf("/api/v1/resource%d", i), noopHandler)
	}
	t.Insert("/users/:id", noopHandler)
	t.Insert("/static/*filepath", noopHandler)
	t.Insert("/api/v1/users/:id/posts/:postID", noopHandler)
	return t
}

func BenchmarkLookup_Static(b *testing.B) {
	tree := buildTree(100)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = tree.Lookup("/api/v1/resource50")
	}
}

func BenchmarkLookup_Param(b *testing.B) {
	tree := buildTree(100)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = tree.Lookup("/users/12345")
	}
}

func BenchmarkLookup_Wildcard(b *testing.B) {
	tree := buildTree(100)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = tree.Lookup("/static/css/main.css")
	}
}

func BenchmarkLookup_TwoParams(b *testing.B) {
	tree := buildTree(100)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = tree.Lookup("/api/v1/users/42/posts/99")
	}
}

func BenchmarkLookup_Miss(b *testing.B) {
	tree := buildTree(100)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = tree.Lookup("/this/does/not/exist")
	}
}

func BenchmarkInsert(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		t := NewTree()
		for j := 0; j < 100; j++ {
			t.Insert(fmt.Sprintf("/api/v1/resource%d", j), noopHandler)
		}
	}
}
