package nativeapp

import "sync"

type richCacheEntry[T any] struct {
	value  T
	weight int
}

// FIFO storage has explicit entry/byte budgets, including original/prepared
// text keys. Eviction only removes rebuildable render data, never user content.
type richCache[T any] struct {
	mu                   sync.Mutex
	items                map[string]richCacheEntry[T]
	order                []string
	head, bytes          int
	maxEntries, maxBytes int
}

func (c *richCache[T]) get(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.items[key]
	return v.value, ok
}
func (c *richCache[T]) set(key string, value T, weight int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.maxEntries <= 0 || c.maxBytes <= 0 || weight > c.maxBytes {
		return
	}
	if c.items == nil {
		c.items = map[string]richCacheEntry[T]{}
	}
	if _, ok := c.items[key]; ok {
		return
	}
	for len(c.items) >= c.maxEntries || c.bytes+weight > c.maxBytes {
		old := c.order[c.head]
		c.head++
		entry := c.items[old]
		delete(c.items, old)
		c.bytes -= entry.weight
		c.order[c.head-1] = ""
	}
	if c.head > 128 {
		n := copy(c.order, c.order[c.head:])
		clear(c.order[n:])
		c.order = c.order[:n]
		c.head = 0
	}
	c.items[key] = richCacheEntry[T]{value, weight}
	c.order = append(c.order, key)
	c.bytes += weight
}
