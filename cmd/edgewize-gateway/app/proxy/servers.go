package proxy

import (
	"github.com/edgewize-io/edgewize/cmd/edgewize-gateway/app/options"
	"k8s.io/klog"
	"sync"
)

type ServerEndpoints struct {
	sync.RWMutex
	servers map[string]string
}

func NewServers() *ServerEndpoints {
	return &ServerEndpoints{
		servers: map[string]string{},
	}
}

func (s *ServerEndpoints) Get(name string) (string, bool) {
	s.RLock()
	defer s.RUnlock()
	ret, ok := s.servers[name]
	if !ok {
		return "", ok
	}
	return ret, ok
}

func (s *ServerEndpoints) LoadConfig(endpoints options.ServerEndpoints) {
	newServers := make(map[string]string, len(endpoints))
	for key, endpoint := range endpoints {
		newServers[key] = endpoint.Clusterip
	}
	klog.V(7).Infof("Rewrite server's endpoints (old: %v) by (new: %v)", s.servers, newServers)
	s.Lock()
	s.servers = newServers
	s.Unlock()
}
