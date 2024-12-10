package micro

import (
	"errors"
	"google.golang.org/grpc"
)

type Register interface {
	Install(service *ServiceNode) error
	Uninstall()
	SustainLease()
	WithRetryBefore(func())
	WithRetryAfter(func())
	WithLog(func(level LogLevel, message string))
}

type Discovery interface {
	GetService(name string) ([]*ServiceNode, error)
	Watcher()
	Unwatch()
}

type LogLevel string

const (
	Info    LogLevel = "info"
	Error   LogLevel = "error"
	Success LogLevel = "success"
)

// ServiceNode 一般适用于服务注册
type ServiceNode struct {
	Name   string          `json:"name"`
	Method map[string]bool `json:"method"`

	Lease int    `json:"lease"`
	AppId string `json:"app_id"`

	Network    string `json:"network"`
	OuterAddr  string `json:"outer_addr"`
	InsideAddr string `json:"inside_addr"`
}

func (s *ServiceNode) ValidMethod(method string) bool {
	if _, ok := s.Method[method]; ok {
		return true
	}
	return false
}

// ServiceConfig 服务注册/服务发现配置
type ServiceConfig struct {
	Mode      bool   `json:"mode" bson:"mode" yaml:"mode" mapstructure:"mode"`
	AppId     string `json:"app_id" bson:"app_id" yaml:"app_id" mapstructure:"app_id"`
	Namespace string `json:"namespace" bson:"namespace" yaml:"namespace" mapstructure:"namespace"`
	MaxRetry  uint32 `json:"max_retry" bson:"max_retry" yaml:"max_retry" mapstructure:"max_retry"`
	TTL       uint32 `json:"ttl" bson:"ttl" yaml:"ttl" mapstructure:"ttl"`

	Network    string `json:"network" bson:"network" yaml:"network" mapstructure:"network"`
	OuterAddr  string `json:"outer_addr" bson:"outer_addr" yaml:"outer_addr" mapstructure:"outer_addr"`
	InsideAddr string `json:"inside_addr" bson:"inside_addr" yaml:"inside_addr" mapstructure:"inside_addr"`
}

// ServiceInstance 一般适用于服务发现
type ServiceInstance map[string][]*ServiceNode

func (s ServiceInstance) GetNodes(service string) ([]*ServiceNode, error) {
	if v, ok := s[service]; ok {
		return v, nil
	}
	return nil, errors.New("there is currently no available node for this service")
}

// NewRegisterService 注册服务集合
func NewRegisterService(raw []*grpc.ServiceDesc, reg Register) []error {
	var errs []error
	for _, desc := range raw {
		node := &ServiceNode{
			Name:   desc.ServiceName,
			Method: make(map[string]bool),
		}

		for _, item := range desc.Methods {
			node.Method[item.MethodName] = true
		}

		if err := reg.Install(node); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
