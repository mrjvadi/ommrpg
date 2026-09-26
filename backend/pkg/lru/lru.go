// Package lru is a tiny thread-safe LRU cache for regenerable data
// (chunks, dungeon floors, profiles).
package lru

import (
	"container/list"
	"sync"
	"time"
)

type entry[K comparable, V any] struct {
	key K
	val V
	exp time.Time
}

type Cache[K comparable, V any] struct {
	mu    sync.Mutex
	max   int
	ttl   time.Duration
	ll    *list.List
	items map[K]*list.Element
}

// New creates a cache; ttl 0 means entries never expire.
func New[K comparable, V any](max int, ttl time.Duration) *Cache[K, V] {
	return &Cache[K, V]{max: max, ttl: ttl, ll: list.New(), items: map[K]*list.Element{}}
}

func (c *Cache[K, V]) Get(k K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var zero V
	el, ok := c.items[k]
	if !ok {
		return zero, false
	}
	e := el.Value.(*entry[K, V])
	if c.ttl > 0 && time.Now().After(e.exp) {
		c.ll.Remove(el)
		delete(c.items, k)
		return zero, false
	}
	c.ll.MoveToFront(el)
	return e.val, true
}

func (c *Cache[K, V]) Put(k K, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	exp := time.Now().Add(c.ttl)
	if el, ok := c.items[k]; ok {
		el.Value = &entry[K, V]{k, v, exp}
		c.ll.MoveToFront(el)
		return
	}
	c.items[k] = c.ll.PushFront(&entry[K, V]{k, v, exp})
	for c.ll.Len() > c.max {
		last := c.ll.Back()
		c.ll.Remove(last)
		delete(c.items, last.Value.(*entry[K, V]).key)
	}
}

func (c *Cache[K, V]) Delete(k K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[k]; ok {
		c.ll.Remove(el)
		delete(c.items, k)
	}
}
