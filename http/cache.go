package http

import (
	"fmt"
	"net"
	"sync"
)

type cacheEntry struct {
	key        uint64
	resp       Response
	prev, next *cacheEntry
}

type Cache struct {
	capacity  int
	mu        sync.RWMutex
	entries   map[uint64]*cacheEntry
	head      *cacheEntry
	tail      *cacheEntry
	evictions uint64
}

type CacheStats struct {
	Capacity  int
	Size      int
	Evictions uint64
}

func NewCache(capacity int) *Cache {
	if capacity < 0 {
		capacity = 0
	}
	return &Cache{
		capacity: capacity,
		entries:  make(map[uint64]*cacheEntry, capacity),
	}
}

const (
	offset64 = 14695981039346656037
	prime64  = 1099511628211
)

func key(ip net.IP) uint64 {
	var h uint64 = offset64
	for _, b := range ip {
		h ^= uint64(b)
		h *= prime64
	}
	return h
}

func keyWithLang(ip net.IP, lang string) uint64 {
	var h uint64 = offset64
	for _, b := range ip {
		h ^= uint64(b)
		h *= prime64
	}
	for i := 0; i < len(lang); i++ {
		h ^= uint64(lang[i])
		h *= prime64
	}
	return h
}

func (c *Cache) removeNode(node *cacheEntry) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		c.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		c.tail = node.prev
	}
	node.prev = nil
	node.next = nil
}

func (c *Cache) addToFront(node *cacheEntry) {
	node.next = c.head
	node.prev = nil
	if c.head != nil {
		c.head.prev = node
	}
	c.head = node
	if c.tail == nil {
		c.tail = node
	}
}

func (c *Cache) moveToFront(node *cacheEntry) {
	if c.head == node {
		return
	}
	c.removeNode(node)
	c.addToFront(node)
}

func (c *Cache) Set(ip net.IP, resp Response) {
	c.SetWithLang(ip, "", resp)
}

func (c *Cache) SetWithLang(ip net.IP, lang string, resp Response) {
	if c.capacity == 0 {
		return
	}
	k := keyWithLang(ip, lang)

	c.mu.Lock()
	defer c.mu.Unlock()

	if current, exists := c.entries[k]; exists {
		current.resp = resp
		c.moveToFront(current)
		return
	}

	var node *cacheEntry
	if len(c.entries) >= c.capacity {
		node = c.tail
		if node != nil {
			c.removeNode(node)
			delete(c.entries, node.key)
			c.evictions++
		} else {
			node = &cacheEntry{}
		}
	} else {
		node = &cacheEntry{}
	}

	node.key = k
	node.resp = resp
	c.entries[k] = node
	c.addToFront(node)
}

func (c *Cache) Get(ip net.IP) (Response, bool) {
	return c.GetWithLang(ip, "")
}

func (c *Cache) GetWithLang(ip net.IP, lang string) (Response, bool) {
	if c.capacity == 0 {
		return Response{}, false
	}
	k := keyWithLang(ip, lang)
	c.mu.RLock()
	defer c.mu.RUnlock()
	node, ok := c.entries[k]
	if !ok {
		return Response{}, false
	}
	return node.resp, true
}

func (c *Cache) Resize(capacity int) error {
	if capacity < 0 {
		return fmt.Errorf("invalid capacity: %d\n", capacity)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.capacity = capacity
	c.evictions = 0
	for len(c.entries) > c.capacity && c.tail != nil {
		node := c.tail
		c.removeNode(node)
		delete(c.entries, node.key)
	}
	return nil
}

func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		Size:      len(c.entries),
		Capacity:  c.capacity,
		Evictions: c.evictions,
	}
}
