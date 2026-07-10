package gm

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorMapping 定义历史哨兵错误到 gRPC status 的映射规则。
type ErrorMapping struct {
	// Target 使用 errors.Is 匹配。
	Target error
	// Code 表示目标 gRPC code。
	Code codes.Code
	// Message 非空时覆盖 err.Error()。
	Message string
}

// ErrorToStatusOptions 定义错误归一化行为。
type ErrorToStatusOptions struct {
	// ValidationCode 用于 protovalidate.ValidationError。
	ValidationCode codes.Code
	// Mappings 将历史哨兵错误映射为明确的 gRPC status code。
	Mappings []ErrorMapping
	// DefaultCode 用于未分类错误。
	DefaultCode codes.Code
	// DefaultMessage 非空时覆盖未分类错误消息。
	DefaultMessage string
	// ExposeDefaultErrorMessage 控制是否向客户端返回未分类错误的 err.Error()。
	ExposeDefaultErrorMessage bool
}

// ErrorToStatusOption 用于定制 ErrorToStatus。
type ErrorToStatusOption func(*ErrorToStatusOptions)

// WithValidationErrorCode 设置 protovalidate 错误使用的 code。
func WithValidationErrorCode(code codes.Code) ErrorToStatusOption {
	return func(options *ErrorToStatusOptions) {
		options.ValidationCode = normalizeErrorStatusCode(code, codes.InvalidArgument)
	}
}

// WithErrorMapping 将历史哨兵错误映射为 gRPC status code。
func WithErrorMapping(target error, code codes.Code, message string) ErrorToStatusOption {
	return func(options *ErrorToStatusOptions) {
		if target == nil {
			return
		}
		options.Mappings = append(options.Mappings, ErrorMapping{
			Target:  target,
			Code:    normalizeErrorStatusCode(code, codes.Internal),
			Message: message,
		})
	}
}

// WithDefaultErrorCode 设置未分类错误使用的 code。
func WithDefaultErrorCode(code codes.Code) ErrorToStatusOption {
	return func(options *ErrorToStatusOptions) {
		options.DefaultCode = normalizeErrorStatusCode(code, codes.Internal)
	}
}

// WithDefaultErrorMessage 设置未分类错误返回给客户端的消息。
func WithDefaultErrorMessage(message string) ErrorToStatusOption {
	return func(options *ErrorToStatusOptions) {
		options.DefaultMessage = message
	}
}

// WithExposeDefaultErrorMessage 控制是否向客户端返回未分类错误的 err.Error()。
func WithExposeDefaultErrorMessage(expose bool) ErrorToStatusOption {
	return func(options *ErrorToStatusOptions) {
		options.ExposeDefaultErrorMessage = expose
	}
}

// ErrorToStatus 在服务出口将错误归一化为 gRPC status error。
//
// 映射顺序：
//  1. 已有 gRPC status error 或实现 GRPCStatus 的错误
//  2. protovalidate.ValidationError，默认映射为 InvalidArgument
//  3. context.Canceled / context.DeadlineExceeded
//  4. 显式配置的历史哨兵错误映射
//  5. 未分类错误，默认映射为 Internal
func ErrorToStatus(options ...ErrorToStatusOption) grpc.UnaryServerInterceptor {
	normalizeOptions := buildErrorToStatusOptions(options...)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		return resp, normalizeErrorToStatus(err, normalizeOptions)
	}
}

// ValidationErrorToInvalidArgument 将 protovalidate.ValidationError 映射为 InvalidArgument。
//
// 该函数仅为兼容旧服务保留；新服务应使用 ErrorToStatus。
func ValidationErrorToInvalidArgument() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		var ve *protovalidate.ValidationError
		if errors.As(err, &ve) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		return resp, err
	}
}

func buildErrorToStatusOptions(options ...ErrorToStatusOption) ErrorToStatusOptions {
	out := ErrorToStatusOptions{
		ValidationCode:            codes.InvalidArgument,
		DefaultCode:               codes.Internal,
		ExposeDefaultErrorMessage: true,
	}
	for _, option := range options {
		if option != nil {
			option(&out)
		}
	}
	out.ValidationCode = normalizeErrorStatusCode(out.ValidationCode, codes.InvalidArgument)
	out.DefaultCode = normalizeErrorStatusCode(out.DefaultCode, codes.Internal)
	return out
}

func normalizeErrorToStatus(err error, options ErrorToStatusOptions) error {
	if err == nil {
		return nil
	}
	if st, ok := status.FromError(err); ok {
		return st.Err()
	}

	var validationErr *protovalidate.ValidationError
	if errors.As(err, &validationErr) {
		return status.Error(options.ValidationCode, err.Error())
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	for _, mapping := range options.Mappings {
		if mapping.Target == nil || !errors.Is(err, mapping.Target) {
			continue
		}
		message := mapping.Message
		if message == "" {
			message = err.Error()
		}
		return status.Error(normalizeErrorStatusCode(mapping.Code, codes.Internal), message)
	}

	message := options.DefaultMessage
	if message == "" && options.ExposeDefaultErrorMessage {
		message = err.Error()
	}
	return status.Error(options.DefaultCode, message)
}

func normalizeErrorStatusCode(code codes.Code, fallback codes.Code) codes.Code {
	if code == codes.OK {
		return fallback
	}
	return code
}
