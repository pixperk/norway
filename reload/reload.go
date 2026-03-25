package reload

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pixperk/norway/dsl"
	"github.com/pixperk/norway/health"
	"github.com/pixperk/norway/router"
	"github.com/pixperk/norway/stats"
)

// SwappableHandler wraps an http.Handler that can be atomically replaced.
// The http.Server holds a pointer to this; on reload we swap the inner handler.
type SwappableHandler struct {
	handler atomic.Value // stores http.Handler
}

func NewSwappableHandler(h http.Handler) *SwappableHandler {
	s := &SwappableHandler{}
	s.handler.Store(h)
	return s
}

func (s *SwappableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.Load().(http.Handler).ServeHTTP(w, r)
}

func (s *SwappableHandler) Swap(h http.Handler) {
	s.handler.Store(h)
}

// BuildFunc is the function that turns a parsed config into a router.
// main.go provides this so the reload package doesn't duplicate wiring logic.
type BuildFunc func(cfg *dsl.Config) (handler http.Handler, checkers []*health.Checker, collector *stats.Collector)

// Reloader holds everything needed to re-read config and swap the live handler.
type Reloader struct {
	configPath string
	handler    *SwappableHandler
	buildFn    BuildFunc

	// track health checkers so we can stop old ones on reload
	mu       sync.Mutex
	checkers []*health.Checker

	// debounce: ignore file events within this window
	lastReload time.Time
	debounce   time.Duration
}

func New(configPath string, handler *SwappableHandler, buildFn BuildFunc) *Reloader {
	return &Reloader{
		configPath: configPath,
		handler:    handler,
		buildFn:    buildFn,
		debounce:   200 * time.Millisecond,
	}
}

// SetCheckers stores the initial set of health checkers (from first boot)
func (rl *Reloader) SetCheckers(checkers []*health.Checker) {
	rl.mu.Lock()
	rl.checkers = checkers
	rl.mu.Unlock()
}

// Reload re-reads the config, rebuilds the handler, and swaps it in.
// If anything fails, the old config stays running.
func (rl *Reloader) Reload() error {
	rl.mu.Lock()
	if time.Since(rl.lastReload) < rl.debounce {
		rl.mu.Unlock()
		return nil
	}
	rl.lastReload = time.Now()
	rl.mu.Unlock()

	log.Println("reload: reading config...")

	data, err := os.ReadFile(rl.configPath)
	if err != nil {
		return fmt.Errorf("reload: failed to read config: %w", err)
	}

	tokens := dsl.NewLexer(string(data)).Tokenize()
	cfg, err := dsl.NewParser(tokens).Parse()
	if err != nil {
		return fmt.Errorf("reload: parse error: %w", err)
	}

	if err := dsl.Validate(cfg); err != nil {
		return fmt.Errorf("reload: validation error: %w", err)
	}

	// build new handler, health checkers, and collector
	newHandler, newCheckers, newCollector := rl.buildFn(cfg)

	// mount stats and reload endpoints on the new router
	// the build function returns the router as the handler
	if r, ok := newHandler.(*router.Router); ok {
		r.AddInternal("/norway/stats", newCollector.Handler())
		r.AddInternal("/norway/reload", rl.APIHandler())
	}

	// stop old health checkers
	rl.mu.Lock()
	for _, c := range rl.checkers {
		c.Stop()
	}
	rl.checkers = newCheckers
	rl.mu.Unlock()

	// atomic swap
	rl.handler.Swap(newHandler)
	log.Println("reload: config applied successfully")
	return nil
}

// WatchFile starts a background goroutine that watches the config file
// for changes and triggers a reload. Editors like vim do write-rename,
// so we watch the directory and filter for the config filename.
func (rl *Reloader) WatchFile() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("reload: failed to create file watcher: %v", err)
		return
	}

	// watch the directory so rename-based saves (vim, etc.) are caught
	dir := filepath.Dir(rl.configPath)
	base := filepath.Base(rl.configPath)

	if err := watcher.Add(dir); err != nil {
		log.Printf("reload: failed to watch %s: %v", dir, err)
		return
	}

	log.Printf("reload: watching %s for changes", rl.configPath)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// only care about writes/creates to our config file
				if filepath.Base(event.Name) != base {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					continue
				}
				if err := rl.Reload(); err != nil {
					log.Printf("reload: %v", err)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("reload: watcher error: %v", err)
			}
		}
	}()
}

// APIHandler returns an http.Handler for POST /norway/reload
func (rl *Reloader) APIHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := rl.Reload(); err != nil {
			log.Printf("reload: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "reload: ok")
	})
}
