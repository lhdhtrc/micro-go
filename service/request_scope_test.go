package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestRequestScopePolicyRegistry(t *testing.T) {
	policies := map[string]RequestScopePolicy{
		"/acme.test.v1.TestService/List": {
			Mode:          RequestScopeModeQuery,
			AllowAppID:    true,
			AllowTenantID: true,
		},
	}
	registry, err := NewRequestScopePolicyRegistry(policies)
	if err != nil {
		t.Fatalf("create registry failed: %v", err)
	}
	delete(policies, "/acme.test.v1.TestService/List")

	policy, ok := registry.Lookup("/acme.test.v1.TestService/List")
	if !ok || policy.Mode != RequestScopeModeQuery || !policy.AllowAppID || !policy.AllowTenantID {
		t.Fatalf("unexpected policy: %+v, ok=%v", policy, ok)
	}
}

func TestRequestScopePolicyRegistryRejectsInvalidPolicy(t *testing.T) {
	_, err := NewRequestScopePolicyRegistry(map[string]RequestScopePolicy{
		"/acme.test.v1.TestService/List": {Mode: RequestScopeModeQuery},
	})
	if !errors.Is(err, ErrRequestScopePolicyInvalid) {
		t.Fatalf("expected invalid policy error, got %v", err)
	}
}

func TestExtractRequestScope(t *testing.T) {
	message := newRequestScopeTestMessage(t, protoreflect.StringKind)
	setRequestScopeTestString(t, message, "app_id", "app-1")
	setRequestScopeTestString(t, message, "tenant_id", "tenant-1")

	scope, err := ExtractRequestScope(message, RequestScopePolicy{
		Mode:          RequestScopeModeQuery,
		AllowAppID:    true,
		AllowTenantID: true,
	})
	if err != nil {
		t.Fatalf("extract scope failed: %v", err)
	}
	if !scope.Explicit() || !scope.AppIDPresent || !scope.TenantIDPresent {
		t.Fatalf("expected explicit app and tenant scope: %+v", scope)
	}
	if scope.AppID != "app-1" || scope.TenantID != "tenant-1" {
		t.Fatalf("unexpected scope values: %+v", scope)
	}
}

func TestExtractRequestScopeIgnoresUnregisteredDimensionAndEmptyValue(t *testing.T) {
	message := newRequestScopeTestMessage(t, protoreflect.StringKind)
	setRequestScopeTestString(t, message, "app_id", "business-app")

	scope, err := ExtractRequestScope(message, RequestScopePolicy{
		Mode:          RequestScopeModeQuery,
		AllowTenantID: true,
	})
	if err != nil {
		t.Fatalf("extract scope failed: %v", err)
	}
	if scope.Explicit() || scope.AppID != "" || scope.AppIDPresent || scope.TenantIDPresent {
		t.Fatalf("unexpected extracted scope: %+v", scope)
	}
}

func TestExtractRequestScopeRejectsNonStringField(t *testing.T) {
	message := newRequestScopeTestMessage(t, protoreflect.Int32Kind)
	_, err := ExtractRequestScope(message, RequestScopePolicy{
		Mode:       RequestScopeModeQuery,
		AllowAppID: true,
	})
	if !errors.Is(err, ErrRequestScopeMessageInvalid) {
		t.Fatalf("expected invalid message error, got %v", err)
	}
}

func TestRequestScopeContextRoundTrip(t *testing.T) {
	value := &RequestScopeContext{
		FullMethod: "/acme.test.v1.TestService/List",
		Requested:  RequestScope{AppID: "app-1", AppIDPresent: true},
	}
	ctx := WithRequestScope(context.Background(), value)
	got, ok := RequestScopeFromContext(ctx)
	if !ok || got != value {
		t.Fatalf("unexpected request scope context: %+v, ok=%v", got, ok)
	}
}

func TestRequestScopeDecisionValidate(t *testing.T) {
	now := time.Unix(1710000000, 0)
	requested := RequestScope{AppID: "app-1", AppIDPresent: true, TenantID: "tenant-1", TenantIDPresent: true}
	decision := RequestScopeDecision{
		Allowed: true,
		Authorized: AuthorizedScope{
			AppIDs:    []string{"app-1"},
			TenantIDs: []string{"tenant-1"},
		},
		ExpiresAt: now.Add(time.Minute),
	}
	if err := decision.Validate(requested, now); err != nil {
		t.Fatalf("expected valid decision, got %v", err)
	}

	decision.Authorized.AppIDs = []string{"app-2"}
	if err := decision.Validate(requested, now); !errors.Is(err, ErrRequestScopeDecisionInvalid) {
		t.Fatalf("expected invalid coverage error, got %v", err)
	}

	decision.Authorized.AppIDs = []string{"app-1"}
	decision.ExpiresAt = now
	if err := decision.Validate(requested, now); !errors.Is(err, ErrRequestScopeDecisionInvalid) {
		t.Fatalf("expected expired decision error, got %v", err)
	}
}

func newRequestScopeTestMessage(t *testing.T, appIDKind protoreflect.Kind) *dynamicpb.Message {
	t.Helper()
	appIDType := descriptorpb.FieldDescriptorProto_TYPE_STRING
	if appIDKind == protoreflect.Int32Kind {
		appIDType = descriptorpb.FieldDescriptorProto_TYPE_INT32
	}
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("request_scope_test.proto"),
		Package: proto.String("acme.test.v1"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Request"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{Name: proto.String("app_id"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: appIDType.Enum()},
				{Name: proto.String("tenant_id"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
			},
		}},
	}, nil)
	if err != nil {
		t.Fatalf("create test descriptor failed: %v", err)
	}
	return dynamicpb.NewMessage(file.Messages().ByName("Request"))
}

func setRequestScopeTestString(t *testing.T, message *dynamicpb.Message, fieldName protoreflect.Name, value string) {
	t.Helper()
	field := message.Descriptor().Fields().ByName(fieldName)
	if field == nil {
		t.Fatalf("field %s not found in %s", fieldName, prototext.Format(message))
	}
	message.Set(field, protoreflect.ValueOfString(value))
}
