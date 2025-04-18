package micro

import "errors"

// ServiceNode 一般适用于服务注册
type ServiceNode struct {
	ProtoCount int             `json:"proto_count"`
	LeaseId    int             `json:"lease_id"`
	RunDate    string          `json:"run_date"`
	Methods    map[string]bool `json:"methods"`

	Network *Network `json:"network"`
	Kernel  *Kernel  `json:"kernel"`
	Meta    *Meta    `json:"meta"`
}

// ParseMethod 解析方法
func (ist *ServiceNode) ParseMethod(s ServiceMethods) {
	for k, _ := range ist.Methods {
		s[k] = ist.Meta.AppId
	}
}

// CheckMethod 检查方法
func (ist *ServiceNode) CheckMethod(sm string) error {
	if _, ok := ist.Methods[sm]; ok {
		return nil
	}
	return errors.New("service node does not have this method")
}
