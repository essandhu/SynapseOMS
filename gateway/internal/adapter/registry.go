package adapter

import "sync"

// AdapterFactory creates a LiquidityProvider from a configuration map.
type AdapterFactory func(config map[string]string) LiquidityProvider

var (
	registryMu sync.RWMutex
	registry   = map[string]AdapterFactory{}
)

// Register adds an adapter factory for a given venue type.
func Register(venueType string, factory AdapterFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[venueType] = factory
}

// Get returns the adapter factory for the given venue type.
func Get(venueType string) (AdapterFactory, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	f, ok := registry[venueType]
	return f, ok
}
