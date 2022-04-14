package geecache

import (
	"fmt"
	"github.com/ballache233/geecache/singleflight"
	"log"
	"sync"
)

type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc 实现了一个接口型函数
type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

type Group struct {
	// 组名
	name string
	// 回调函数
	getter Getter
	// 主存储结构
	mainCache cache
	// 绑定的HTTP server，用于通过其他节点获取数据
	peers PeerPicker
	// 防止缓存击穿
	loader *singleflight.Group
}

var (
	mu     sync.RWMutex
	groups = map[string]*Group{}
)

// NewGroup 创建一个新的group,group是cache的核心数据结构
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}
	groups[name] = g
	return g
}

// RegisterPeers 将group和本地的HTTPPool绑定，为了选择节点
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// GetGroup 根据group名获取group
func GetGroup(name string) *Group {
	mu.RLock()
	defer mu.RUnlock()
	return groups[name]
}

// Get group的查询
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	// 调用group的cache成员的get方法，然后这个get方法是调用的底层数据结构lru.cache的get方法
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeeCache] hit")
		return v, nil
		// 没查询到就从本地或者其他节点载入(load)
	} else {
		return g.load(key)
	}
}

// 从本地或者其他节点载入
func (g *Group) load(key string) (value ByteView, err error) {
	// each key is only fetched once (either locally or remotely)
	// regardless of the number of concurrent callers.
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[GeeCache] Failed to get from peer", err)
			}
		}

		return g.getLocally(key)
	})

	if err == nil {
		return viewi.(ByteView), nil
	}
	return
}

// 从其他节点载入
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: cloneBytes(bytes)}, nil
}

// 从本地数据库载入
func (g *Group) getLocally(key string) (ByteView, error) {
	// 调用回调函数
	bytes, err := g.getter.(GetterFunc)(key)
	// 本地没有找到就返回一个空的数据和一个error
	if err != nil {
		return ByteView{}, err
	}
	// 找到了就将[]byte类型封装成存储的结构ByteView
	value := ByteView{b: cloneBytes(bytes)}
	// 调用populateCache方法，将数据加入cache
	g.populateCache(key, value)
	return value, nil
}

// 更新cache
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}
