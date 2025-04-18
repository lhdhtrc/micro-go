package micro

import "errors"

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
