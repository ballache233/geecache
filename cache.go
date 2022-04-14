package geecache

import (
	"github.com/ballache233/geecache/lru"
	"sync"
)

type cache struct {
	// 互斥锁
	mu sync.Mutex
	// 底层存储结构
	lru *lru.Cache
	// 缓存的最大空间，对应底层结构的maxBytes
	cacheBytes int64
}

func (c *cache) add(key string, value ByteView) {
	// 加互斥锁
	c.mu.Lock()
	defer c.mu.Unlock()
	// 此处判断c.lru == nil属于lazy initializing，提高性能，减少程序内存要求
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}
	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}
	return
}
