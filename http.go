package geecache

import (
	"fmt"
	"github.com/ballache233/geecache/consistenthash"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// PeerGetter 接口相当于HTTP的客户端，用于向远程的HTTP服务端发送请求
type PeerGetter interface {
	Get(group string, key string) ([]byte, error)
}

// PeerPicker 接口中的PickPeer方法是根据传入的key选择相应节点PeerGetter
type PeerPicker interface {
	PickPeer(key string) (PeerGetter, bool)
}

// 结构体实现了PeerGetter接口
type httpGetter struct {
	baseURL string
}

func (h *httpGetter) Get(group string, key string) ([]byte, error) {
	// 根据自己的baseURL(地址)，访问的group和key创建url
	u := "http://" + fmt.Sprintf("%v%v/%v", h.baseURL, url.QueryEscape(group), url.QueryEscape(key))
	// 向指定url发送get请求
	res, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %v", res.Status)
	}
	// 读入响应中的数据并返回
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}

	return bytes, nil
}

var _ PeerGetter = (*httpGetter)(nil)

const (
	defaultBasePath = "/_geecache/"
	defaultReplicas = 50
)

// HTTPPool 实现了客户端和服务端的功能，可以根据key查自己，如果自己没有，则发送请求查其他节点
type HTTPPool struct {
	// 自己节点的地址
	self string
	// 访问的api
	basePath string
	// 加锁保护peers和httpGetters
	mu sync.Mutex
	// 可以根据Key找到应该访问的节点地址
	peers *consistenthash.Map
	// 可以根据地址找到对应的httpGetter并发送请求
	httpGetters map[string]*httpGetter
}

// NewHTTPPool 创建一个HTTPPool实例，与下面的Set方法一起实现初始化
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Set 设置所有可选择节点的信息，放到一致性哈希中
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, nil)
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	p.peers.Add(peers...)
	for _, peer := range peers {
		httpGetter := new(httpGetter)
		httpGetter.baseURL = peer + p.basePath
		p.httpGetters[peer] = httpGetter
	}
}

// PickPeer 缓存数据在其他节点，方法的返回值是PeerGetter，调用者直接调用PeerGetter的Get方法即可得到需要的数据
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 拿到key对应的地址peer
	peer := p.peers.Get(key)
	// 如果peer和本地地址相同，即peer == p.self，不去其他节点找，直接在本地载入；当Key == ""时，peer可能为""
	if peer == p.self || peer == "" {
		return nil, false
	}
	p.Log("pick peer %s", peer)
	// 返回key对应地址的PeerGetter
	return p.httpGetters[peer], true
}

func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[serve %s] %s", p.self, fmt.Sprintf(format, v...))
}

func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log(r.Method, r.URL.Path)
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	groupName := parts[0]
	key := parts[1]
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group", http.StatusBadRequest)
		return
	}

	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(view.ByteSlice())
}
