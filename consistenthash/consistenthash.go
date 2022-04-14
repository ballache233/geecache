package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(data []byte) uint32

type Map struct {
	hash     Hash
	replicas int
	keys     []int
	hashMap  map[int]string
}

// New Map构造函数，支持自定义hash函数，如果fn == nil，则默认使用ChecksumIEEE为hash函数
func New(replicas int, fn Hash) *Map {
	res := &Map{
		hash:     fn,
		replicas: replicas,
		hashMap:  make(map[int]string),
	}
	if fn == nil {
		res.hash = crc32.ChecksumIEEE
	}
	return res
}

// Add 实现添加真实节点
func (m *Map) Add(keys ...string) {
	for _, str := range keys {
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(str + strconv.Itoa(i))))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = str
		}
	}
	sort.Ints(m.keys)
}

func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	return m.hashMap[m.keys[idx%len(m.keys)]]
}
