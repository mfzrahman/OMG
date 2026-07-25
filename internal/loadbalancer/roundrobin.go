package loadbalancer

import (
	"errors"
	"sync/atomic"

	"github.com/omg/omg/internal/model"
)

// RoundRobin distributes requests across backends weighted by their
// Weight field. Heavier backends receive proportionally more requests.
type RoundRobin struct {
	counter atomic.Uint64
}

// NewRoundRobin creates a weighted round-robin load balancer.
func NewRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

func (rr *RoundRobin) Select(backends []*model.BackendWithState) (*model.BackendWithState, error) {
	if len(backends) == 0 {
		return nil, errors.New("no backends available")
	}

	// Build a flat weighted list.
	type entry struct {
		backend *model.BackendWithState
		weight  int
	}

	flat := make([]entry, 0, len(backends)*2)
	totalWeight := 0
	for _, b := range backends {
		w := b.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
		flat = append(flat, entry{backend: b, weight: w})
	}

	if totalWeight == 0 {
		return nil, errors.New("total weight is zero")
	}

	idx := int(rr.counter.Add(1)) % totalWeight
	cumulative := 0
	for _, e := range flat {
		cumulative += e.weight
		if idx < cumulative {
			return e.backend, nil
		}
	}

	return flat[0].backend, nil // should never reach here
}
