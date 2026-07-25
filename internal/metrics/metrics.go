package metrics

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Collector aggregates gateway metrics in a thread-safe structure and
// exposes them in Prometheus text exposition format.
type Collector struct {
	mu sync.RWMutex

	// Per-route counters
	reqCount   map[string]int64 // key: route_id
	errCount   map[string]int64 // key: route_id
	latencySum map[string]int64 // key: route_id (nanoseconds)
	latencyCnt map[string]int64 // key: route_id

	// Per-backend counters
	backendReqCount   map[string]int64 // key: backend_id
	backendErrCount   map[string]int64 // key: backend_id
	backendLatencySum map[string]int64 // key: backend_id
	backendLatencyCnt map[string]int64 // key: backend_id

	// Active connections gauge
	activeConns map[string]int64 // key: route_id
}

// NewCollector creates a new metrics collector.
func NewCollector() *Collector {
	return &Collector{
		reqCount:          make(map[string]int64),
		errCount:          make(map[string]int64),
		latencySum:        make(map[string]int64),
		latencyCnt:        make(map[string]int64),
		backendReqCount:   make(map[string]int64),
		backendErrCount:   make(map[string]int64),
		backendLatencySum: make(map[string]int64),
		backendLatencyCnt: make(map[string]int64),
		activeConns:       make(map[string]int64),
	}
}

// RecordRequest records a completed proxy request.
func (c *Collector) RecordRequest(routeID, backendID string, statusCode int, latency time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.reqCount[routeID]++
	c.latencySum[routeID] += latency.Nanoseconds()
	c.latencyCnt[routeID]++

	c.backendReqCount[backendID]++
	c.backendLatencySum[backendID] += latency.Nanoseconds()
	c.backendLatencyCnt[backendID]++

	if statusCode >= 500 {
		c.errCount[routeID]++
		c.backendErrCount[backendID]++
	}
}

// IncActiveConns increments the active connection count for a route.
func (c *Collector) IncActiveConns(routeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeConns[routeID]++
}

// DecActiveConns decrements the active connection count for a route.
func (c *Collector) DecActiveConns(routeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeConns[routeID]--
	if c.activeConns[routeID] < 0 {
		c.activeConns[routeID] = 0
	}
}

// ServeHTTP renders metrics in Prometheus text exposition format.
func (c *Collector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)

	// Route request counters
	for routeID, count := range c.reqCount {
		fmt.Fprintf(w, `omg_route_requests_total{route="%s"} %d`+"\n", routeID, count)
	}
	// Route error counters
	for routeID, count := range c.errCount {
		fmt.Fprintf(w, `omg_route_errors_total{route="%s"} %d`+"\n", routeID, count)
	}
	// Route latency averages
	for routeID := range c.latencySum {
		sum := c.latencySum[routeID]
		cnt := c.latencyCnt[routeID]
		avg := float64(0)
		if cnt > 0 {
			avg = float64(sum) / float64(cnt) / 1e6 // ms
		}
		fmt.Fprintf(w, `omg_route_latency_ms{route="%s"} %.3f`+"\n", routeID, avg)
	}
	// Active connections
	for routeID, count := range c.activeConns {
		fmt.Fprintf(w, `omg_route_active_connections{route="%s"} %d`+"\n", routeID, count)
	}
	// Backend request counters
	for backendID, count := range c.backendReqCount {
		fmt.Fprintf(w, `omg_backend_requests_total{backend="%s"} %d`+"\n", backendID, count)
	}
	// Backend error counters
	for backendID, count := range c.backendErrCount {
		fmt.Fprintf(w, `omg_backend_errors_total{backend="%s"} %d`+"\n", backendID, count)
	}
}
