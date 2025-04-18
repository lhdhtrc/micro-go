package micro

import (
	"fmt"
	"google.golang.org/grpc"
)

// NewRegisterService 注册服务集合
func NewRegisterService(raw []*grpc.ServiceDesc, reg Register) []error {
	node := new(ServiceNode)
	node.ProtoCount = len(raw)
	node.Methods = make(map[string]bool)

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
