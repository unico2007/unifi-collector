// Package collector defines the vendor-neutral contracts of the framework: the
// Collector interface every collector implements, the Source capability
// interfaces vendor adapters satisfy, the Sink interfaces exporters satisfy,
// and a Registry that holds collectors for the scheduler to run.
package collector

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Collector performs one unit of data collection. Implementations are expected
// to be safe to call repeatedly and to honor ctx cancellation/deadlines.
type Collector interface {
	// Name is a stable, unique identifier, e.g. "devices". It is matched
	// against the config's collectors map to resolve the schedule.
	Name() string
	// Collect performs a single collection cycle.
	Collect(ctx context.Context) error
}

// Registry is a concurrency-safe set of collectors keyed by name. The scheduler
// reads from it to know what to run; wiring code writes to it at startup.
type Registry struct {
	mu         sync.RWMutex
	collectors map[string]Collector
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{collectors: make(map[string]Collector)}
}

// Register adds c. It errors on a nil collector, an empty name, or a duplicate.
func (r *Registry) Register(c Collector) error {
	if c == nil {
		return fmt.Errorf("registry: nil collector")
	}
	name := c.Name()
	if name == "" {
		return fmt.Errorf("registry: collector with empty name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.collectors[name]; exists {
		return fmt.Errorf("registry: collector %q already registered", name)
	}
	r.collectors[name] = c
	return nil
}

// Get returns the collector registered under name.
func (r *Registry) Get(name string) (Collector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.collectors[name]
	return c, ok
}

// All returns every registered collector, ordered by name for determinism.
func (r *Registry) All() []Collector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.collectors))
	for name := range r.collectors {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Collector, 0, len(names))
	for _, name := range names {
		out = append(out, r.collectors[name])
	}
	return out
}
