package balance

// LeastConn picks the healthy backend with the fewest active connections.
// When two backends have equal counts, the first one wins (stable ordering).
type LeastConn struct {
	backends []*Backend
}

func NewLeastConn(backends []*Backend) *LeastConn {
	return &LeastConn{backends: backends}
}

// Next scans all backends and returns the healthy one with the lowest ActiveConns.
// Returns nil if all backends are down.
//
// Not lock-free like RoundRobin, but the scan is O(n) where n = number of backends
// (usually < 10), so it's fast enough. ActiveConns is atomic so the read is safe.
func (l *LeastConn) Next() *Backend {
	var pick *Backend
	var minConns int64

	for _, b := range l.backends {
		if !b.Healthy.Load() {
			continue
		}
		conns := b.ActiveConns.Load()
		if pick == nil || conns < minConns {
			pick = b
			minConns = conns
		}
	}
	return pick
}

func (l *LeastConn) All() []*Backend {
	return l.backends
}
