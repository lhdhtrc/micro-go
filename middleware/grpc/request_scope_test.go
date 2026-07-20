package gm

import (
	"context"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

const requestScopeTestFullMethod = "/acme.test.v1.TestService/List"

type requestScopeAuthorizerFunc func(context.Context, service.RequestScopeAuthorization) (service.RequestScopeDecision, error)

func (f requestScopeAuthorizerFunc) AuthorizeRequestScope(ctx context.Context, request service.RequestScopeAuthorization) (service.RequestScopeDecision, error) {
	return f(ctx, request)
}

func TestNewRequestScopeUnaryInterceptorAllowsExplicitScope(t *testing.T) {
	registry := newRequestScopeTestRegistry(t)
	message := newRequestScopeMiddlewareMessage(t)
	setRequestScopeMiddlewareField(t, message, "app_id", "app-1")
	setRequestScopeMiddlewareField(t, message, "tenant_id", "tenant-1")

	authorizerCalled := false
	interceptor := NewRequestScopeUnaryInterceptor(RequestScopeInterceptorOptions{
		Registry: registry,
		Authorizer: requestScopeAuthorizerFunc(func(ctx context.Context, request service.RequestScopeAuthorization) (service.RequestScopeDecision, error) {
			authorizerCalled = true
			if request.FullMethod != requestScopeTestFullMethod || request.Requested.AppID != "app-1" || request.Requested.TenantID != "tenant-1" {
				t.Fatalf("unexpected authorization request: %+v", request)
			}
			return service.RequestScopeDecision{
				Allowed:       true,
				DecisionID:    "scope-decision-1",
				PolicyVersion: "v1",
				Authorized: service.AuthorizedScope{
					AppIDs:    []string{"app-1"},
					TenantIDs: []string{"tenant-1"},
				},
			}, nil
		}),
	})

	response, err := interceptor(context.Background(), message, &grpc.UnaryServerInfo{FullMethod: requestScopeTestFullMethod}, func(ctx context.Context, req any) (any, error) {
		value, ok := service.RequestScopeFromContext(ctx)
		if !ok || value.Decision == nil {
			t.Fatalf("expected authorized request scope context: %+v, ok=%v", value, ok)
		}
		if value.Decision.DecisionID != "scope-decision-1" || !value.Requested.Explicit() {
			t.Fatalf("unexpected request scope context: %+v", value)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}
	if response != "ok" || !authorizerCalled {
		t.Fatalf("unexpected result: response=%v authorizer_called=%v", response, authorizerCalled)
	}
}

func TestNewRequestScopeUnaryInterceptorUsesDefaultScopeWithoutAuthorizer(t *testing.T) {
	interceptor := NewRequestScopeUnaryInterceptor(RequestScopeInterceptorOptions{
		Registry: newRequestScopeTestRegistry(t),
	})
	message := newRequestScopeMiddlewareMessage(t)

	_, err := interceptor(context.Background(), message, &grpc.UnaryServerInfo{FullMethod: requestScopeTestFullMethod}, func(ctx context.Context, req any) (any, error) {
		value, ok := service.RequestScopeFromContext(ctx)
		if !ok || value.Requested.Explicit() || value.Decision != nil {
			t.Fatalf("unexpected default request scope context: %+v, ok=%v", value, ok)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("expected default scope to pass, got %v", err)
	}
}

func TestNewRequestScopeUnaryInterceptorDeniesExplicitScope(t *testing.T) {
	message := newRequestScopeMiddlewareMessage(t)
	setRequestScopeMiddlewareField(t, message, "app_id", "app-denied")
	interceptor := NewRequestScopeUnaryInterceptor(RequestScopeInterceptorOptions{
		Registry: newRequestScopeTestRegistry(t),
		Authorizer: requestScopeAuthorizerFunc(func(ctx context.Context, request service.RequestScopeAuthorization) (service.RequestScopeDecision, error) {
			return service.RequestScopeDecision{Allowed: false, Reason: "scope denied"}, nil
		}),
	})

	handlerCalled := false
	_, err := interceptor(context.Background(), message, &grpc.UnaryServerInfo{FullMethod: requestScopeTestFullMethod}, func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return nil, nil
	})
	if status.Code(err) != codes.PermissionDenied || handlerCalled {
		t.Fatalf("expected permission denied before handler, err=%v handler_called=%v", err, handlerCalled)
	}
}

func TestNewRequestScopeUnaryInterceptorFailsClosedWithoutAuthorizer(t *testing.T) {
	message := newRequestScopeMiddlewareMessage(t)
	setRequestScopeMiddlewareField(t, message, "tenant_id", "tenant-1")
	interceptor := NewRequestScopeUnaryInterceptor(RequestScopeInterceptorOptions{
		Registry: newRequestScopeTestRegistry(t),
	})

	_, err := interceptor(context.Background(), message, &grpc.UnaryServerInfo{FullMethod: requestScopeTestFullMethod}, func(ctx context.Context, req any) (any, error) {
		return nil, nil
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", err)
	}
}

func TestNewRequestScopeUnaryInterceptorRejectsInvalidAllowDecision(t *testing.T) {
	now := time.Unix(1710000000, 0)
	message := newRequestScopeMiddlewareMessage(t)
	setRequestScopeMiddlewareField(t, message, "app_id", "app-1")
	interceptor := NewRequestScopeUnaryInterceptor(RequestScopeInterceptorOptions{
		Registry: newRequestScopeTestRegistry(t),
		Now:      func() time.Time { return now },
		Authorizer: requestScopeAuthorizerFunc(func(ctx context.Context, request service.RequestScopeAuthorization) (service.RequestScopeDecision, error) {
			return service.RequestScopeDecision{
				Allowed:    true,
				ExpiresAt:  now.Add(-time.Second),
				Authorized: service.AuthorizedScope{AppIDs: []string{"app-1"}},
			}, nil
		}),
	})

	_, err := interceptor(context.Background(), message, &grpc.UnaryServerInfo{FullMethod: requestScopeTestFullMethod}, func(ctx context.Context, req any) (any, error) {
		return nil, nil
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected invalid decision to fail closed, got %v", err)
	}
}

func TestNewRequestScopeUnaryInterceptorIgnoresUnregisteredMethod(t *testing.T) {
	interceptor := NewRequestScopeUnaryInterceptor(RequestScopeInterceptorOptions{
		Registry: newRequestScopeTestRegistry(t),
	})
	response, err := interceptor(context.Background(), &struct{}{}, &grpc.UnaryServerInfo{FullMethod: "/acme.test.v1.TestService/Info"}, func(ctx context.Context, req any) (any, error) {
		if _, ok := service.RequestScopeFromContext(ctx); ok {
			t.Fatal("unregistered method must not receive request scope context")
		}
		return "ok", nil
	})
	if err != nil || response != "ok" {
		t.Fatalf("unexpected unregistered method result: response=%v err=%v", response, err)
	}
}

func newRequestScopeTestRegistry(t *testing.T) *service.RequestScopePolicyRegistry {
	t.Helper()
	registry, err := service.NewRequestScopePolicyRegistry(map[string]service.RequestScopePolicy{
		requestScopeTestFullMethod: {
			Mode:          service.RequestScopeModeQuery,
			AllowAppID:    true,
			AllowTenantID: true,
		},
	})
	if err != nil {
		t.Fatalf("create request scope registry failed: %v", err)
	}
	return registry
}

func newRequestScopeMiddlewareMessage(t *testing.T) *dynamicpb.Message {
	t.Helper()
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("request_scope_middleware_test.proto"),
		Package: proto.String("acme.test.v1"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Request"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("app_id"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
				{Name: proto.String("tenant_id"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
			},
		}},
	}, nil)
	if err != nil {
		t.Fatalf("create middleware test descriptor failed: %v", err)
	}
	return dynamicpb.NewMessage(file.Messages().ByName("Request"))
}

func setRequestScopeMiddlewareField(t *testing.T, message *dynamicpb.Message, fieldName protoreflect.Name, value string) {
	t.Helper()
	field := message.Descriptor().Fields().ByName(fieldName)
	if field == nil {
		t.Fatalf("field %s not found", fieldName)
	}
	message.Set(field, protoreflect.ValueOfString(value))
}
