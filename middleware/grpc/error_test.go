package gm

import (
	"context"
	"errors"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/fireflycore/go-micro/werror"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidationErrorToInvalidArgumentInterceptor_MapsValidationError(t *testing.T) {
	interceptor := ValidationErrorToInvalidArgument()

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, &protovalidate.ValidationError{}
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected %v, got %v (%s)", codes.InvalidArgument, st.Code(), st.Message())
	}
}

func TestValidationErrorToInvalidArgumentInterceptor_PassesThroughOtherErrors(t *testing.T) {
	interceptor := ValidationErrorToInvalidArgument()
	origErr := errors.New("x")

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, origErr
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, origErr) {
		t.Fatalf("expected original error, got %T: %v", err, err)
	}
}

func TestErrorToStatus_MapsWrappedError(t *testing.T) {
	interceptor := ErrorToStatus()

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, werror.InvalidArgument("验证码已过期/不存在")
	})
	assertStatusCode(t, err, codes.InvalidArgument, "验证码已过期/不存在")
}

func TestErrorToStatus_MapsJoinedWrappedError(t *testing.T) {
	interceptor := ErrorToStatus()

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, errors.Join(werror.NotFound("用户不存在"), errors.New("lookup failed"))
	})
	assertStatusCode(t, err, codes.NotFound, "用户不存在\nlookup failed")
}

func TestErrorToStatus_MapsValidationError(t *testing.T) {
	interceptor := ErrorToStatus()

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, &protovalidate.ValidationError{}
	})
	assertStatusCode(t, err, codes.InvalidArgument, "")
}

func TestErrorToStatus_PassesThroughStatusError(t *testing.T) {
	interceptor := ErrorToStatus()

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.Unauthenticated, "token is required")
	})
	assertStatusCode(t, err, codes.Unauthenticated, "token is required")
}

func TestErrorToStatus_MapsContextErrors(t *testing.T) {
	interceptor := ErrorToStatus()

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, context.Canceled
	})
	assertStatusCode(t, err, codes.Canceled, context.Canceled.Error())

	_, err = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, context.DeadlineExceeded
	})
	assertStatusCode(t, err, codes.DeadlineExceeded, context.DeadlineExceeded.Error())
}

func TestErrorToStatus_MapsLegacySentinelError(t *testing.T) {
	errVerifyCodeExpired := errors.New("验证码已过期/不存在")
	interceptor := ErrorToStatus(
		WithErrorMapping(errVerifyCodeExpired, codes.InvalidArgument, "验证码已过期/不存在"),
	)

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, errVerifyCodeExpired
	})
	assertStatusCode(t, err, codes.InvalidArgument, "验证码已过期/不存在")
}

func TestErrorToStatus_MapsUnclassifiedErrorToInternal(t *testing.T) {
	interceptor := ErrorToStatus()

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, errors.New("redis unavailable")
	})
	assertStatusCode(t, err, codes.Internal, "redis unavailable")
}

func TestErrorToStatus_HidesUnclassifiedMessageWhenConfigured(t *testing.T) {
	interceptor := ErrorToStatus(
		WithExposeDefaultErrorMessage(false),
		WithDefaultErrorMessage("internal server error"),
	)

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		return nil, errors.New("redis password leaked")
	})
	assertStatusCode(t, err, codes.Internal, "internal server error")
}

func assertStatusCode(t *testing.T, err error, code codes.Code, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != code {
		t.Fatalf("expected %v, got %v (%s)", code, st.Code(), st.Message())
	}
	if st.Message() != message {
		t.Fatalf("expected message %q, got %q", message, st.Message())
	}
}
