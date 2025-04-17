package micro

import (
	"errors"
	"fmt"
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
	LeaseId    int `json:"lease_id"`
	ProtoCount int `json:"proto_count"`

	Env     string `json:"env"`
	AppId   string `json:"app_id"`
	Version string `json:"version"`
	RunDate string `json:"run_date"`

	Network *Network        `json:"network"`
	Methods map[string]bool `json:"methods"`
}

// ParseMethod 解析方法
func (ist *ServiceNode) ParseMethod(s ServiceMethods) {
	for k, _ := range ist.Methods {
		s[k] = ist.AppId
	}
}

// CheckMethod 检查方法
func (ist *ServiceNode) CheckMethod(sm string) error {
	if _, ok := ist.Methods[sm]; ok {
		return nil
	}
	return errors.New("service node does not have this method")
}

// ServiceConf 服务注册/服务发现配置
type ServiceConf struct {
	// 命名控件
	Namespace string `json:"namespace" bson:"namespace" yaml:"namespace" mapstructure:"namespace"`
	// 网卡
	Network *Network `json:"network"`

	// 最大重试次数
	MaxRetry uint32 `json:"max_retry" bson:"max_retry" yaml:"max_retry" mapstructure:"max_retry"`
	// 心跳间隔
	TTL uint32 `json:"ttl" bson:"ttl" yaml:"ttl" mapstructure:"ttl"`
}

// ServiceDiscover 服务发现
type ServiceDiscover map[string][]*ServiceNode

func (s ServiceDiscover) GetNodes(appId string) ([]*ServiceNode, error) {
	if v, ok := s[appId]; ok {
		return v, nil
	}
	return nil, errors.New("service node not exists")
}

// ServiceMethods 服务方法
type ServiceMethods map[string]string

func (s ServiceMethods) GetAppId(sm string) (string, error) {
	if v, ok := s[sm]; ok {
		return v, nil
	}
	return "", errors.New("service method not exists")
}

// NewRegisterService 注册服务集合
func NewRegisterService(raw []*grpc.ServiceDesc, reg Register) []error {
	node := new(ServiceNode)
	node.ProtoCount = len(raw)

	var errs []error
	for _, desc := range raw {
		for _, item := range desc.Methods {
			node.Methods[fmt.Sprintf("/%s/%s", desc.ServiceName, item.MethodName)] = true
		}
	}

	if err := reg.Install(node); err != nil {
		errs = append(errs, err)
	}

	return errs
}
