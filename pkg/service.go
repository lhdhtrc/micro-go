package micro

import (
	"context"
	"errors"
)

type ServiceNode struct {
	Name          string `json:"name"`
	Lease         string `json:"lease"`
	OuterNetIp    string `json:"outer_net_ip"`
	InternalNetIp string `json:"internal_net_ip"`

	Method map[string]string `json:"method"`
}

func (s *ServiceNode) ValidMethod(method string) bool {
	if _, ok := s.Method[method]; ok {
		return true
	}
	return false
}

type ServiceInstance map[string][]*ServiceNode

func (s ServiceInstance) GetNodes(service string) ([]*ServiceNode, error) {
	if v, ok := s[service]; ok {
		return v, nil
	}
	return nil, errors.New("there is currently no available node for this service")
}

type Register interface {
	Install(ctx context.Context, service *ServiceNode) error
	Uninstall(ctx context.Context, service *ServiceNode) error
}

type Discovery interface {
	GetService(ctx context.Context, name string) ([]*ServiceNode, error)
	Watcher(ctx context.Context, namespace string) (Watcher, error)
}

type Watcher interface {
	Next() ([]*ServiceNode, error)
	Stop() error
}
