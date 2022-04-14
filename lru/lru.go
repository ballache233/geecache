package lru

import (
	"container/list"
)

type Cache struct {
	// 双向链表
	ll *list.List
	// 允许使用的最大内存
	maxBytes int64
	// 已经使用的内存
	nbytes int64
	// 通过键查找双向链表的节点
	cache map[string]*list.Element
	// 某条记录被移除时的回调函数回调函数
	OnEvicted func(key string, value Value)
}

// 链表中节点存储的数据类型
type entry struct {
	key   string
	value Value
}

type Value interface {
	Len() int
}

// New 实现实例化
func New(maxBytes int64, OnEvicted func(string, Value)) *Cache {
	return &Cache{
		ll:        list.New(),
		maxBytes:  maxBytes,
		nbytes:    0,
		cache:     make(map[string]*list.Element),
		OnEvicted: OnEvicted,
	}
}

// Get 按照key查找
func (c *Cache) Get(key string) (value Value, ok bool) {
	if element, ok := c.cache[key]; ok {
		kv := element.Value.(*entry)
		value = kv.value
		// 约定优先级最高的时front, 最低的是back
		c.ll.MoveToFront(element)
		return value, ok
	}
	return
}

// RemoveOldest 淘汰优先级最低的节点
func (c *Cache) RemoveOldest() {
	element := c.ll.Back()
	if element != nil {
		// 取出待移除数据，删除cache的索引
		kv := element.Value.(*entry)
		delete(c.cache, kv.key)
		// 更新nbytes
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		// 调用回调函数
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
		c.ll.Remove(element)
	}
}

// Add 更新/修改缓存
func (c *Cache) Add(key string, value Value) {
	// 如果存在
	if element, ok := c.cache[key]; ok {
		// 更新也算访问，所以移到最高级优先级
		c.ll.MoveToFront(element)
		kv := element.Value.(*entry)
		// 更新nbytes
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
		// 如果不存在
	} else {
		// 创建一个存储结构
		kv := &entry{
			key:   key,
			value: value,
		}
		// 创建一个节点并插入到最高优先级位置
		element := c.ll.PushFront(kv)
		// 更新map和nbytes
		c.nbytes += int64(len(key)) + int64(kv.value.Len())
		c.cache[key] = element
	}
	// 插入后可能导致缓存溢出，循环移除最低优先级的缓存，直至nbytes符合要求，如果maxBytes = 0代表不限制缓存大小
	for c.maxBytes != 0 && c.nbytes > c.maxBytes {
		c.RemoveOldest()
	}
}
