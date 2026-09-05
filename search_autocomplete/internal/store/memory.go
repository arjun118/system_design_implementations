package store

import "github.com/arjun118/autocomplete/internal/suggest"

type MemoryCache struct {
	PI map[string][]suggest.Reco
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		PI: make(map[string][]suggest.Reco),
	}
}

func (c *MemoryCache) Get(prefix string) ([]suggest.Reco, bool) {
	recos, exists := c.PI[prefix]
	if !exists {
		return nil, false
	}
	return recos, true

}
func (c *MemoryCache) Set(prefix string, suggestions []suggest.Reco) {
	c.PI[prefix] = suggestions
}
