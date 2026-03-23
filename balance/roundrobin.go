package balance

import "sync/atomic"

// RoundRobin distributes requests evenly across backends.
// Uses an atomic counter to avoid locks. Skips unhealthy backends.
type RoundRobin struct {
	backends []*Backend
	counter  atomic.Uint64
}

func NewRoundRobin(backends []*Backend) *RoundRobin {
	return &RoundRobin{backends: backends}
}

// Next picks the next healthy backend in rotation.
// Tries at most len(backends) times to find a healthy one.
// Returns nil if all backends are down.
func (r *RoundRobin) Next() *Backend {
	n := len(r.backends)
	if n == 0 {
		return nil
	}

	// atomic increment gives us a unique index per request, no locks needed
	start := int(r.counter.Add(1) - 1)

	for i := 0; i < n; i++ {
		b := r.backends[(start+i)%n]
		if b.Healthy.Load() {
			return b
		}
	}
	return nil
}

func (r *RoundRobin) All() []*Backend {
	return r.backends
}
