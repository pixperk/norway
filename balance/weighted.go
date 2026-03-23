package balance

import "sync/atomic"

// Weighted distributes requests proportionally based on backend weights.
// A backend with weight 3 gets 3x the traffic of a backend with weight 1.
// Internally it expands backends into a flat list and round-robins over it.
//
// Example: backends [A(weight=3), B(weight=1)]
// expanded: [A, A, A, B]
// requests hit: A, A, A, B, A, A, A, B, ...
type Weighted struct {
	expanded []*Backend
	all      []*Backend
	counter  atomic.Uint64
}

func NewWeighted(backends []*Backend) *Weighted {
	var expanded []*Backend
	for _, b := range backends {
		for j := 0; j < b.Weight; j++ {
			expanded = append(expanded, b)
		}
	}
	return &Weighted{expanded: expanded, all: backends}
}

// Next picks the next healthy backend based on weight distribution.
// Tries at most len(expanded) times to find a healthy one.
func (w *Weighted) Next() *Backend {
	n := len(w.expanded)
	if n == 0 {
		return nil
	}

	start := int(w.counter.Add(1) - 1)

	for i := 0; i < n; i++ {
		b := w.expanded[(start+i)%n]
		if b.Healthy.Load() {
			return b
		}
	}
	return nil
}

func (w *Weighted) All() []*Backend {
	return w.all
}
