package http

import (
	"container/list"
	"fmt"
	"hash"
	"hash/fnv"
	"net"
	"sync"
)

type Cache struct {
	capacity  int
	mu        sync.RWMutex
	entries   map[uint64]*list.Element
	values    *list.List
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
		entries:  make(map[uint64]*list.Element),
		values:   list.New(),
	}
}

// 哈希对象池，减少内存分配
var hashPool = sync.Pool{
	New: func() interface{} {
		return fnv.New64a()
	},
}

func key(ip net.IP) uint64 {
	h := hashPool.Get().(hash.Hash64)
	defer func() {
		h.Reset()
		hashPool.Put(h)
	}()
	h.Write(ip)
	return h.Sum64()
}

func keyWithLang(ip net.IP, lang string) uint64 {
	h := hashPool.Get().(hash.Hash64)
	defer func() {
		h.Reset()
		hashPool.Put(h)
	}()
	h.Write(ip)
	if lang != "" {
		h.Write([]byte(lang))
	}
	return h.Sum64()
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

	// 优化：先检查是否已存在，避免不必要的操作
	if current, exists := c.entries[k]; exists {
		// 更新现有条目，移到最后
		c.values.Remove(current)
		c.entries[k] = c.values.PushBack(resp)
		return
	}

	// 优化：当达到容量时进行清理
	if len(c.entries) >= c.capacity {
		// 计算需要清理的条目数
		minEvictions := len(c.entries) - c.capacity + 1

		// 为了性能，批量清理更多条目（最多25%），但至少清理到容量以下
		evictCount := minEvictions
		batchSize := c.capacity / 4
		if batchSize > minEvictions && c.capacity > 4 {
			evictCount = batchSize
		}

		evicted := 0
		for el := c.values.Front(); el != nil && evicted < evictCount; {
			// 找到并删除对应的map条目
			for k, v := range c.entries {
				if v == el {
					delete(c.entries, k)
					break
				}
			}
			next := el.Next()
			c.values.Remove(el)
			el = next
			evicted++
		}
		c.evictions += uint64(evicted)
	}

	c.entries[k] = c.values.PushBack(resp)
}

func (c *Cache) Get(ip net.IP) (Response, bool) {
	return c.GetWithLang(ip, "")
}

func (c *Cache) GetWithLang(ip net.IP, lang string) (Response, bool) {
	k := keyWithLang(ip, lang)
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.entries[k]
	if !ok {
		return Response{}, false
	}
	return r.Value.(Response), true
}

func (c *Cache) Resize(capacity int) error {
	if capacity < 0 {
		return fmt.Errorf("invalid capacity: %d\n", capacity)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.capacity = capacity
	c.evictions = 0
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
