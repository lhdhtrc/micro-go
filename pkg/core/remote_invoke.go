package micro

import (
	"errors"
)

// InvokeResponse 远程调用响应
type InvokeResponse struct {
	Code    uint32 `json:"code"`
	Message string `json:"message"`
}

// WithRemoteInvoke 远程调用
func WithRemoteInvoke[T any](callFunc func() (T, *InvokeResponse, error)) (T, error) {
	// 执行服务调用
	data, res, err := callFunc()
	if err != nil {
		return data, err
	}

	// 检查响应状态码
	if res.Code != 200 {
		return data, errors.New(res.Message)
	}

	return data, nil
}
