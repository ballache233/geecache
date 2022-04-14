package lru

import (
	"reflect"
	"testing"
)

type String string

func (d String) Len() int {
	return len(d)
}

// 测试Get方法
func TestCache_Get(t *testing.T) {
	lru := New(int64(0), nil)
	lru.Add("key1", String("1234"))
	// 存入的信息找不到或者与存入信息不符，不通过测试
	if v, ok := lru.Get("key1"); !ok || string(v.(String)) != "1234" {
		t.Fatalf("cache hit key1=1234 failed")
	}
	// 找到了没有存入的信息，不通过测试
	if _, ok := lru.Get("key2"); ok {
		t.Fatalf("cache miss key2 failed")
	}
}

// 测试是否有溢出淘汰功能
func TestCache_RemoveOldest(t *testing.T) {
	k1, k2, k3 := "key1", "key2", "k3"
	v1, v2, v3 := "value1", "value2", "v3"
	cap := len(k1 + k2 + v1 + v2)
	lru := New(int64(cap), nil)
	lru.Add(k1, String(v1))
	lru.Add(k2, String(v2))
	// k1, k2插入后缓存刚好满，插入K3必然淘汰k1
	lru.Add(k3, String(v3))

	// 如果找到了被淘汰的k1，测试不通过
	if _, ok := lru.Get("key1"); ok || len(lru.cache) != 2 {
		t.Fatalf("Removeoldest key1 failed")
	}
}

// 测试回调函数
func TestOnEvicted(t *testing.T) {
	keys := make([]string, 0)
	callback := func(key string, value Value) {
		keys = append(keys, key)
	}
	lru := New(int64(10), callback)
	lru.Add("key1", String("123456"))
	lru.Add("k2", String("k2"))
	// 插入k3,k4后key1,k2会被淘汰，根据回调函数的定义，两个Key会存在keys切片中
	lru.Add("k3", String("k3"))
	lru.Add("k4", String("k4"))

	expect := []string{"key1", "k2"}

	// 如果keys切片中没有key1,k2或者顺序不对，测试不通过
	if !reflect.DeepEqual(expect, keys) {
		t.Fatalf("Call OnEvicted failed, expect keys equals to %s", expect)
	}
}
