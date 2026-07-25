package loadbalancer

import "github.com/omg/omg/internal/model"

// Balancer selects a backend from a list of candidates.
type Balancer interface {
	Select(backends []*model.BackendWithState) (*model.BackendWithState, error)
}
