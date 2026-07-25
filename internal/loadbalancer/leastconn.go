package loadbalancer

import (
	"errors"
	"sync"

	"github.com/omg/omg/internal/model"
)

// LeastConn selects the backend with the fewest active connections.
type LeastConn struct {
	mu sync.Mutex
}

// NewLeastConn creates a least-connections load balancer.
func NewLeastConn() *LeastConn {
	return &LeastConn{}
}

func (lc *LeastConn) Select(backends []*model.BackendWithState) (*model.BackendWithState, error) {
	if len(backends) == 0 {
		return nil, errors.New("no backends available")
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()

	var best *model.BackendWithState
	for _, b := range backends {
		if best == nil || b.ActiveConns < best.ActiveConns {
			best = b
		}
	}
	if best == nil {
		return nil, errors.New("no backend selected")
	}
	return best, nil
}
