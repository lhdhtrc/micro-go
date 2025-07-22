package micro

import (
	"errors"
	"reflect"
)

// RemoteResponse 定义远程调用响应的标准接口
type RemoteResponse[T any] interface {
	GetCode() uint32    // 获取状态码
	GetMessage() string // 获取消息文本
	GetData() T         // 获取业务数据
}

// WithRemoteInvoke 执行远程调用并处理标准化响应
// T: 业务数据类型
// R: 响应类型，必须实现 RemoteResponse[T] 接口
func WithRemoteInvoke[T any, R RemoteResponse[T]](callFunc func() (R, error)) (T, error) {
	var zero T // 创建数据类型的零值

	// 1. 执行远程调用
	resp, err := callFunc()
	if err != nil {
		return zero, err
	}

	// 2. 检查响应对象是否有效
	respValue := reflect.ValueOf(resp)
	if respValue.Kind() == reflect.Ptr && respValue.IsNil() {
		return zero, errors.New("remote response is nil")
	}

	// 3. 检查状态码
	if code := resp.GetCode(); code != 200 {
		msg := resp.GetMessage()
		if msg == "" {
			msg = "remote call failed"
		}
		return zero, errors.New(msg)
	}

	// 4. 获取业务数据
	data := resp.GetData()

	return data, nil
}
