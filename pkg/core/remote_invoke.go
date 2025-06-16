package micro

import (
	"errors"
)

// WithRemoteInvoke 远程调用
func WithRemoteInvoke[T any](callFunc func() (data T, code uint32, message string, err error)) (T, error) {
	// 执行服务调用
	data, code, message, err := callFunc()
	if err != nil {
		return data, err
	}

	// 检查响应状态码
	if code != 200 {
		return data, errors.New(message)
	}

	return data, nil
}
