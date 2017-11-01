package main

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

// DNSMsg DNS cache message
type DNSMsg struct {
	Hit    int64    `label:"query cache hit count"`
	Expire int64    `label:"dns query result cache exprice, zero is never expire"`
	Msg    *dns.Msg `label:"dns query result"`
}

// Cache memory base dns query cache
// TODO 计划要添加一个后台线程，对查询次数多的进行后台更新来加速整体性能
type Cache struct {
	MaxCount int                `label:"number of dns query cache item, zero is not limit"`
	MinTTL   int64              `label:"min cache time, zero is not limit"`
	MaxTTL   int64              `label:"max cache time, zero is not limit"`
	mu       *sync.RWMutex      `label:"query cache read & write lock"`
	backend  map[string]*DNSMsg `label:"dns query cache store"`
}

// Get get query cache
func (c *Cache) Get(key string) (*dns.Msg, bool) {
	c.mu.RLock()
	msg, ok := c.backend[key]
	c.mu.RUnlock()
	if !ok {
		return nil, ok
	}

	if msg.Expire > 0 && msg.Expire < time.Now().Unix() {
		c.Remove(key)
		return nil, false
	}

	return msg.Msg, true

}

// Set Set query cache
func (c *Cache) Set(key string, msg *DNSMsg) bool {
	if c.Full() {
		return false
	}

	c.mu.Lock()
	c.backend[key] = msg
	c.mu.Unlock()

	return true
}

// Remove remove query cache
func (c *Cache) Remove(key string) {
	c.mu.Lock()
	delete(c.backend, key)
	c.mu.Unlock()
}

// Reset reset dns query cache result
func (c *Cache) Reset() {
	c.mu.Lock()
	c.backend = make(map[string]*DNSMsg, 100)
	c.mu.Unlock()
}

// Exists cache is exists
func (c *Cache) Exists(key string) bool {
	c.mu.RLock()
	_, ok := c.backend[key]
	c.mu.RUnlock()

	return ok
}

// Full cache is full
// if Maxcount is zero. the cache will never be full.
func (c *Cache) Full() bool {
	return c.MaxCount > 0 && c.Length() >= c.MaxCount
}

// Length cache length
func (c *Cache) Length() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.backend)
}
