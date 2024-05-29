//Implement Least Recently Used cache to check for duplication transaction ID

package resolvers

import (
	"container/list"
)

type TrxCache struct {
	capacity int
	data     map[string]*list.Element
	order    *list.List
}

func NewTrxCache(capacity int) *TrxCache {
	return &TrxCache{
		capacity: capacity,
		data:     make(map[string]*list.Element),
		order:    list.New(),
	}
}

func (c *TrxCache) Put(key string) {
	if ele, exists := c.data[key]; exists {
		// If key already exists, move it to the front
		c.order.MoveToFront(ele)
		return
	}

	// If at capacity, remove the oldest entry
	if c.order.Len() == c.capacity {
		c.evictOldest()
	}

	// Add new entry to the front of the list and to the map
	ele := c.order.PushFront(key)
	c.data[key] = ele
}

func (c *TrxCache) Exists(key string) bool {
	_, exists := c.data[key]
	return exists
}

func (c *TrxCache) evictOldest() {
	// Remove the oldest entry (tail of the list)
	oldest := c.order.Back()
	if oldest != nil {
		c.order.Remove(oldest)
		delete(c.data, oldest.Value.(string))
	}
}
