package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := ":3000"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": "hello from backend",
			"path":    r.URL.Path,
			"method":  r.Method,
			"host":    r.Host,
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	fmt.Printf("test backend listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
