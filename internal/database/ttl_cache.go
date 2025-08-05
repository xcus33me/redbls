package ttlcache

import (
	"sort"
	"sync"
	"time"
)

type cacheEntry struct {
	value string
	expiresAt time.Time
}

type Cache struct {
	mu sync.RWMutex
	entries map[string]cacheEntry
	exit chan struct{}
	once sync.Once
}

func New(interval time.Duration) *Cache {
	c := &Cache{
		entries: make(map[string]cacheEntry),
		exit: make(chan struct{}),
	}

	go c.CleanupLoop(interval)

	return c
}

func (c *Cache) Add(key string, value string, ttl time.Duration) {
	entry := cacheEntry{
		value: value,
		expiresAt: time.Now().Add(ttl),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = entry
}

func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	var zero string
	if !exists {
		return zero, false
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return zero, false
	}

	return entry.value, true
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

func (c *Cache) CleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
			case <-ticker.C:
				c.CleanupExpired()
			case <-c.exit:
				return
		}
	}
}

func (c *Cache) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	for k, v := range c.entries {
		if now.After(v.expiresAt) {
			delete(c.entries, k)
		}
	}
}

func (c *Cache) Close() {
	c.once.Do(func() {
		close(c.exit)
	})
}

type dumpEntry struct {
	key   string
	value string
	ttl   time.Duration
}

func (c *Cache) Dump() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	var entries []dumpEntry

	for k, v := range c.entries {
		ttl := v.expiresAt.Sub(now)
		if ttl <= 0 {
			continue
		}
		entries = append(entries, dumpEntry{
			key:   k,
			value: v.value,
			ttl:   ttl,
		})
	}

	if len(entries) == 0 {
		return "(cache is empty or all keys expired)"
	}

	// Сортировка по TTL (по возрастанию)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ttl < entries[j].ttl
	})

	// Формируем результат
	result := ""
	for _, e := range entries {
		result += e.key + " = \"" + e.value + "\" (ttl: " + e.ttl.String() + ")\n"
	}

	return result
}
