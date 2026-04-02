package adapter

import "sync"

// AdapterFactory creates a LiquidityProvider from a configuration map.
type AdapterFactory func(config map[string]string) LiquidityProvider

var (
	registryMu  sync.RWMutex
	registry    = map[string]AdapterFactory{}
	instancesMu sync.RWMutex
	instances   = map[string]LiquidityProvider{}
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

// All returns a copy of all registered adapter factories.
func All() map[string]AdapterFactory {
	registryMu.RLock()
	defer registryMu.RUnlock()
	cp := make(map[string]AdapterFactory, len(registry))
	for k, v := range registry {
		cp[k] = v
	}
	return cp
}

// RegisterInstance tracks an instantiated adapter by its venue ID.
func RegisterInstance(venueID string, provider LiquidityProvider) {
	instancesMu.Lock()
	defer instancesMu.Unlock()
	instances[venueID] = provider
}

// GetInstance returns an instantiated adapter by venue ID.
func GetInstance(venueID string) (LiquidityProvider, bool) {
	instancesMu.RLock()
	defer instancesMu.RUnlock()
	p, ok := instances[venueID]
	return p, ok
}

// ListInstances returns all registered adapter instances regardless of status.
func ListInstances() map[string]LiquidityProvider {
	instancesMu.RLock()
	defer instancesMu.RUnlock()
	cp := make(map[string]LiquidityProvider, len(instances))
	for k, v := range instances {
		cp[k] = v
	}
	return cp
}

// ListConnected returns all registered adapter instances that are currently connected.
func ListConnected() []LiquidityProvider {
	instancesMu.RLock()
	defer instancesMu.RUnlock()
	var connected []LiquidityProvider
	for _, p := range instances {
		if p.Status() == Connected {
			connected = append(connected, p)
		}
	}
	return connected
}
