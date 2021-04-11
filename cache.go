package main

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

// CacheItem DNS cache message item
type CacheItem struct {
	Hit    int64    `label:"query cache hit count"`
	Expire int64    `label:"dns query result cache exprice, zero is never expire"`
	Msg    *dns.Msg `label:"dns query result"`
}

// Cache memory base dns query cache
// TODO 计划要添加一个后台线程，对查询次数多的进行后台更新来加速整体性能
type Cache struct {
	MaxCount int                   `label:"number of dns query cache item, zero is not limit"`
	MinTTL   int64                 `label:"min cache time, zero is not limit"`
	MaxTTL   int64                 `label:"max cache time, zero is not limit"`
	mu       *sync.RWMutex         `label:"query cache read & write lock"`
	backend  map[string]*CacheItem `label:"dns query cache store"`
}

// Get get query cache
func (c *Cache) Get(key string) (*dns.Msg, error) {
	var err error
	var msg *dns.Msg

	c.mu.RLock()
	if item, ok := c.backend[key]; ok {
		msg = item.Msg.Copy()
		if item.Expire > 0 && item.Expire < time.Now().Unix() {
			err = ErrCacheExpire
		}
	} else {
		err = ErrNotFound
	}
	c.mu.RUnlock()

	return msg, err
}

// Set Set query cache
func (c *Cache) Set(key string, msg *CacheItem) bool {
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

// IsExpire check dns cache is expire
func (c *Cache) IsExpire(key string) bool {
	var flag bool

	c.mu.RLock()
	if item, ok := c.backend[key]; ok {
		if item.Expire > 0 && item.Expire < time.Now().Unix() {
			flag = true
		}
	}
	c.mu.RUnlock()

	return flag
}

func (c *Cache) GC() {
	var expire = time.Now().Unix() - 86400

	c.mu.Lock()
	for k, v := range c.backend {
		if v.Expire > 0 && v.Expire < expire {
			delete(c.backend, k)
		}
	}
	c.mu.Unlock()
}

// Reset reset dns query cache result
func (c *Cache) Reset() {
	c.mu.Lock()
	c.backend = make(map[string]*CacheItem, 10240)
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
