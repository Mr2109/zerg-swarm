package inference

import (
	"sync"

	"github.com/Mr2109/zerg-swarm/core/internal/store"
)

// RuntimeRegistry 运行时注册表——按机器名管理 Runtime。
type RuntimeRegistry struct {
	mu      sync.RWMutex
	local   *LocalRuntime
	remotes map[string]*RemoteRuntime
}

// NewRegistry 创建空注册表。
func NewRegistry(local *LocalRuntime) *RuntimeRegistry {
	return &RuntimeRegistry{
		local:   local,
		remotes: make(map[string]*RemoteRuntime),
	}
}

// RegisterRemote 注册远端 Runtime。
func (r *RuntimeRegistry) RegisterRemote(host, url, token string, st *store.Store) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remotes[host] = NewRemoteRuntime(host, url, token, st)
}

// Get 按机器名获取 Runtime（local 特殊处理）。
func (r *RuntimeRegistry) Get(host string) Runtime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if host == "local" {
		return r.local
	}
	if rt, ok := r.remotes[host]; ok {
		return rt
	}
	return nil
}

// Local 返回本机 Runtime。
func (r *RuntimeRegistry) Local() *LocalRuntime {
	return r.local
}

// All 返回所有 Runtime（local + remotes）。
func (r *RuntimeRegistry) All() map[string]Runtime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]Runtime, len(r.remotes)+1)
	out["local"] = r.local
	for host, rt := range r.remotes {
		out[host] = rt
	}
	return out
}
