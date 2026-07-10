// Package werror 提供与传输协议无关的错误包装能力。
package werror

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error 表示业务代码可直接返回的结构化错误。
//
// Error 使用 gRPC code 携带跨协议错误语义，同时保留原始原因、稳定原因标识和扩展字段。
// 它实现 GRPCStatus，gRPC 可直接识别，go-micro 中间件也可在服务出口统一处理。
type Error struct {
	code    codes.Code
	message string
	reason  string
	details map[string]string
	cause   error
}

// Option 用于定制 Error。
type Option func(*Error)

// WithReason 设置稳定、可供程序读取的错误原因标识。
func WithReason(reason string) Option {
	return func(err *Error) {
		err.reason = reason
	}
}

// WithDetail 增加一个可供程序读取的错误详情字段。
func WithDetail(key string, value string) Option {
	return func(err *Error) {
		if key == "" {
			return
		}
		if err.details == nil {
			err.details = make(map[string]string)
		}
		err.details[key] = value
	}
}

// WithDetails 批量增加可供程序读取的错误详情字段。
func WithDetails(details map[string]string) Option {
	return func(err *Error) {
		for key, value := range details {
			if key == "" {
				continue
			}
			if err.details == nil {
				err.details = make(map[string]string, len(details))
			}
			err.details[key] = value
		}
	}
}

// WithCause 记录底层错误原因。
func WithCause(cause error) Option {
	return func(err *Error) {
		err.cause = cause
	}
}

// New 创建一个结构化错误。
func New(code codes.Code, message string, options ...Option) *Error {
	err := &Error{
		code:    normalizeCode(code),
		message: message,
	}
	for _, option := range options {
		if option != nil {
			option(err)
		}
	}
	return err
}

// Wrap 使用明确的状态码和对外消息包装底层错误。
func Wrap(code codes.Code, cause error, message string, options ...Option) *Error {
	if cause == nil {
		return New(code, message, options...)
	}
	options = append(options, WithCause(cause))
	return New(code, message, options...)
}

// Error 返回可读错误消息。
func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	return err.Message()
}

// Unwrap 返回底层错误原因。
func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

// Is 支持 errors.Is 按指针或相同 reason 与 code 匹配哨兵错误。
func (err *Error) Is(target error) bool {
	if err == nil {
		return target == nil
	}
	targetErr, ok := target.(*Error)
	if !ok || targetErr == nil {
		return false
	}
	if err == targetErr {
		return true
	}
	return targetErr.reason != "" && err.reason == targetErr.reason && err.code == targetErr.code
}

// GRPCStatus 返回对应的 gRPC status。
func (err *Error) GRPCStatus() *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}
	return status.New(err.code, err.Message())
}

// Code 返回错误携带的 gRPC code。
func (err *Error) Code() codes.Code {
	if err == nil {
		return codes.OK
	}
	return err.code
}

// Message 返回可读错误消息。
func (err *Error) Message() string {
	if err == nil {
		return ""
	}
	if err.message != "" {
		return err.message
	}
	if err.cause != nil {
		return err.cause.Error()
	}
	return err.code.String()
}

// Reason 返回稳定、可供程序读取的错误原因标识。
func (err *Error) Reason() string {
	if err == nil {
		return ""
	}
	return err.reason
}

// Details 返回错误详情字段的副本。
func (err *Error) Details() map[string]string {
	if err == nil || len(err.details) == 0 {
		return nil
	}
	out := make(map[string]string, len(err.details))
	for key, value := range err.details {
		out[key] = value
	}
	return out
}

// FromError 从错误链中查找 Error。
func FromError(err error) (*Error, bool) {
	if err == nil {
		return nil, false
	}
	var wrappedErr *Error
	if errors.As(err, &wrappedErr) {
		return wrappedErr, true
	}
	return nil, false
}

// CodeOf 返回 err 携带的 gRPC code；普通错误返回 codes.Unknown。
func CodeOf(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	if st, ok := status.FromError(err); ok {
		return st.Code()
	}
	return codes.Unknown
}

// StatusOf 返回 err 携带的 gRPC status。
func StatusOf(err error) (*status.Status, bool) {
	if err == nil {
		return status.New(codes.OK, ""), true
	}
	if st, ok := status.FromError(err); ok {
		return st, true
	}
	return nil, false
}

// StatusErrorOf 将结构化错误或已有 gRPC 错误转换为 status error。
func StatusErrorOf(err error) error {
	if err == nil {
		return nil
	}
	if st, ok := StatusOf(err); ok {
		return st.Err()
	}
	return status.Error(codes.Unknown, err.Error())
}

func normalizeCode(code codes.Code) codes.Code {
	if code == codes.OK {
		return codes.Unknown
	}
	return code
}

// Canceled 创建 codes.Canceled 错误。
func Canceled(message string, options ...Option) *Error {
	return New(codes.Canceled, message, options...)
}

// Unknown 创建 codes.Unknown 错误。
func Unknown(message string, options ...Option) *Error {
	return New(codes.Unknown, message, options...)
}

// InvalidArgument 创建 codes.InvalidArgument 错误。
func InvalidArgument(message string, options ...Option) *Error {
	return New(codes.InvalidArgument, message, options...)
}

// DeadlineExceeded 创建 codes.DeadlineExceeded 错误。
func DeadlineExceeded(message string, options ...Option) *Error {
	return New(codes.DeadlineExceeded, message, options...)
}

// NotFound 创建 codes.NotFound 错误。
func NotFound(message string, options ...Option) *Error {
	return New(codes.NotFound, message, options...)
}

// AlreadyExists 创建 codes.AlreadyExists 错误。
func AlreadyExists(message string, options ...Option) *Error {
	return New(codes.AlreadyExists, message, options...)
}

// PermissionDenied 创建 codes.PermissionDenied 错误。
func PermissionDenied(message string, options ...Option) *Error {
	return New(codes.PermissionDenied, message, options...)
}

// ResourceExhausted 创建 codes.ResourceExhausted 错误。
func ResourceExhausted(message string, options ...Option) *Error {
	return New(codes.ResourceExhausted, message, options...)
}

// FailedPrecondition 创建 codes.FailedPrecondition 错误。
func FailedPrecondition(message string, options ...Option) *Error {
	return New(codes.FailedPrecondition, message, options...)
}

// Aborted 创建 codes.Aborted 错误。
func Aborted(message string, options ...Option) *Error {
	return New(codes.Aborted, message, options...)
}

// OutOfRange 创建 codes.OutOfRange 错误。
func OutOfRange(message string, options ...Option) *Error {
	return New(codes.OutOfRange, message, options...)
}

// Unimplemented 创建 codes.Unimplemented 错误。
func Unimplemented(message string, options ...Option) *Error {
	return New(codes.Unimplemented, message, options...)
}

// Internal 创建 codes.Internal 错误。
func Internal(message string, options ...Option) *Error {
	return New(codes.Internal, message, options...)
}

// Unavailable 创建 codes.Unavailable 错误。
func Unavailable(message string, options ...Option) *Error {
	return New(codes.Unavailable, message, options...)
}

// DataLoss 创建 codes.DataLoss 错误。
func DataLoss(message string, options ...Option) *Error {
	return New(codes.DataLoss, message, options...)
}

// Unauthenticated 创建 codes.Unauthenticated 错误。
func Unauthenticated(message string, options ...Option) *Error {
	return New(codes.Unauthenticated, message, options...)
}
