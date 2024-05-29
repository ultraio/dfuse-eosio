//Implement Least Recently Used cache to check for duplication transaction ID

package resolvers

import (
	"container/list"
)

type TrxCache struct {
	capacity int
	data     map[string]bool
	order    *list.List
}

func NewTrxCache(capacity int) *TrxCache {
	return &TrxCache{
		capacity: capacity,
		data:     make(map[string]bool),
		order:    list.New(),
	}
}

func (c *TrxCache) Put(key string) {
	if _, exists := c.data[key]; exists {
		// If key already exists, update value and move it to the front
		c.data[key] = true
		c.moveToFront(key)
		return
	}

	// If at capacity, remove the oldest entry
	if c.order.Len() == c.capacity {
		c.evictOldest()
	}

	// Add new entry to the map and the front of the list
	c.data[key] = true
	c.order.PushFront(key)
}

func (c *TrxCache) Get(key string) (interface{}, bool) {
	if value, exists := c.data[key]; exists {
		// Move accessed item to the front
		c.moveToFront(key)
		return value, true
	}
	return nil, false
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

func (c *TrxCache) moveToFront(key string) {
	// Move accessed or updated key to the front of the list
	for e := c.order.Front(); e != nil; e = e.Next() {
		if e.Value.(string) == key {
			c.order.MoveToFront(e)
			break
		}
	}
}
