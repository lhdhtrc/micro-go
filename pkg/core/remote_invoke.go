package micro

import (
	"context"
	"errors"
)

// InvokeResponse 远程调用响应
type InvokeResponse struct {
	Code    uint32 `json:"code"`
	Message string `json:"message"`
}

// WithRemoteInvoke 远程调用
func WithRemoteInvoke[T any](ctx context.Context, invoke func(context.Context) (T, *InvokeResponse, error)) (T, error) {
	// 执行服务调用
	data, res, err := invoke(ctx)
	if err != nil {
		return data, err
	}

	// 检查响应状态码
	if res.Code != 200 {
		return data, errors.New(res.Message)
	}

	return data, nil
}
