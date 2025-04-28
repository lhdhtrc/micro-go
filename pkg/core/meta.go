package micro

import (
	"errors"
	"fmt"
	"google.golang.org/grpc/metadata"
)

// Meta 服务元信息
type Meta struct {
	Env     string `json:"env"`
	AppId   string `json:"app_id"`
	Version string `json:"version"`
}

// ParseMetaKey 解析元信息key
func ParseMetaKey(md *metadata.MD, key string) (string, error) {
	val := md.Get(key)

	if len(val) == 0 {
		return "", errors.New(fmt.Sprintf("%s parse error", key))
	}

	return val[0], nil
}
