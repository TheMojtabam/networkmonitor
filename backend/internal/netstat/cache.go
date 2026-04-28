package netstat

import (
	"sync"
	"time"
)

// Cache maintains state for rate calculations
type Cache struct {
	mu             sync.RWMutex
	lastInterfaces []InterfaceStats
	lastUpdated    time.Time
	minInterval    time.Duration
}

// NewCache creates a new statistics cache
func NewCache(minInterval time.Duration) *Cache {
	return &Cache{
		minInterval: minInterval,
	}
}

// GetInterfacesWithRates collects current stats and calculates rates
func (c *Cache) GetInterfacesWithRates() ([]InterfaceStats, []InterfaceRates, error) {
	current, err := CollectInterfaces()
	if err != nil {
		return nil, nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var rates []InterfaceRates
	now := time.Now()

	// Calculate rates if we have previous data and enough time passed
	if len(c.lastInterfaces) > 0 && now.Sub(c.lastUpdated) >= c.minInterval {
		rates = CalculateRates(c.lastInterfaces, current)
	}

	// Update cache
	c.lastInterfaces = current
	c.lastUpdated = now

	return current, rates, nil
}

// GetLastInterfaces returns the last cached interface stats (without collecting new ones)
func (c *Cache) GetLastInterfaces() []InterfaceStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make([]InterfaceStats, len(c.lastInterfaces))
	copy(result, c.lastInterfaces)
	return result
}

// Reset clears the cache
func (c *Cache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.lastInterfaces = nil
	c.lastUpdated = time.Time{}
}
