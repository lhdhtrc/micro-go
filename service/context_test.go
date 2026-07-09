package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/metadata"
)

func TestBuildContext(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.AppLanguage, "zh-CN",
		constant.Session, "session-1",
		constant.UserId, "user-1",
		constant.AppId, "app-1",
		constant.TenantId, "tenant-1",
		constant.SubjectType, constant.SubjectTypeUser,
		constant.InvokeAppId, "app-1",
		constant.TargetAppId, "order-app",
		constant.RouteMethod, constant.RequestMethodGrpcString,
		constant.RoutePath, "/acme.order.v1.OrderService/List",
		constant.TargetMethod, constant.RequestMethodGrpcString,
		constant.TargetPath, "/acme.order.v1.OrderService/List",
		constant.DecisionId, "decision-1",
		constant.AuthzSign, "signed-context",
		constant.OrgIds, "org-1",
		constant.OrgIds, "org-2",
		constant.PostIds, "post-1",
		constant.RoleIds, "role-1",
	))

	value := BuildContext(ctx, BuildContextOptions{})
	if value == nil {
		t.Fatal("expected service context, got nil")
	}
	if value.UserId != "user-1" || value.AppId != "app-1" || value.TenantId != "tenant-1" {
		t.Fatalf("unexpected identity fields: %+v", value)
	}
	if value.AppLanguage != "zh-CN" || value.Session != "session-1" {
		t.Fatalf("unexpected app context fields: %+v", value)
	}
	if value.SubjectType != constant.SubjectTypeUser || value.InvokeAppId != "app-1" || value.TargetAppId != "order-app" {
		t.Fatalf("unexpected authz identity fields: %+v", value)
	}
	if value.RouteMethod != constant.RequestMethodGrpcString || value.RoutePath != "/acme.order.v1.OrderService/List" {
		t.Fatalf("unexpected api fields: %+v", value)
	}
	if value.TargetMethod != constant.RequestMethodGrpcString || value.TargetPath != "/acme.order.v1.OrderService/List" {
		t.Fatalf("unexpected target fields: %+v", value)
	}
	if value.DecisionId != "decision-1" {
		t.Fatalf("unexpected decision id: %+v", value)
	}
	if value.AuthzSignJWS != "signed-context" {
		t.Fatalf("unexpected authz sign token: %+v", value)
	}
	if value.UserContext == nil || value.UserContext.AppId != "app-1" || value.UserContext.Session != "session-1" {
		t.Fatalf("unexpected grouped user context: %+v", value)
	}
	if value.DecisionContext == nil || value.DecisionContext.TargetAppId != "order-app" {
		t.Fatalf("unexpected grouped decision context: %+v", value)
	}
	if value.DecisionContext.TargetMethod != constant.RequestMethodGrpcString || value.DecisionContext.TargetPath != "/acme.order.v1.OrderService/List" {
		t.Fatalf("unexpected grouped decision target fields: %+v", value.DecisionContext)
	}
	if value.DecisionContext.InvokeAppId != "app-1" {
		t.Fatalf("unexpected grouped decision invoke fields: %+v", value.DecisionContext)
	}
	if value.InvokeServiceAppId != "" {
		t.Fatalf("expected user subject not to derive invoke service app id: %+v", value)
	}
	if value.TargetServiceAppId != "order-app" {
		t.Fatalf("unexpected target service app id: %+v", value)
	}
	if len(value.OrgIds) != 2 || len(value.PostIds) != 1 || len(value.RoleIds) != 1 {
		t.Fatalf("unexpected scope fields: %+v", value)
	}
}

func TestBuildContext_UsesLocalServiceIdentityOptions(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.ServiceAppId, "forged-service-app",
		constant.ServiceInstanceId, "forged-instance",
	))

	value := BuildContext(ctx, BuildContextOptions{
		ServiceAppId:      "local-service-app",
		ServiceInstanceId: "local-instance",
	})
	if value.ServiceAppId != "local-service-app" || value.ServiceInstanceId != "local-instance" {
		t.Fatalf("expected local service identity options to win, got %+v", value)
	}
}

func TestBuildContext_IgnoresIncomingServiceIdentityWithoutOptions(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.ServiceAppId, "forged-service-app",
		constant.ServiceInstanceId, "forged-instance",
	))

	value := BuildContext(ctx, BuildContextOptions{})
	if value.ServiceAppId != "" || value.ServiceInstanceId != "" {
		t.Fatalf("expected incoming service identity to be ignored without options, got %+v", value)
	}
}

func TestBuildContext_DoesNotUseAppIdAsInvokeFallback(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.AppId, "user-app",
	))

	value := BuildContext(ctx, BuildContextOptions{})
	if value.AppId != "user-app" || value.InvokeAppId != "" {
		t.Fatalf("expected user app id to stay separate from invoke app id: %+v", value)
	}
}

func TestBuildContext_BuildsInvokeServiceAppIdForServiceSubject(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.SubjectType, constant.SubjectTypeService,
		constant.InvokeAppId, "service-a",
		constant.TargetAppId, "service-b",
	))

	value := BuildContext(ctx, BuildContextOptions{})
	if value.InvokeServiceAppId != "service-a" {
		t.Fatalf("expected service subject to build invoke service app id: %+v", value)
	}
	if value.UserContext != nil {
		t.Fatalf("expected service subject without user fields to keep user context empty: %+v", value.UserContext)
	}
}

func TestBuildVerifiedContext_UsesServiceAppIdAsTargetExpectation(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	raw := signTestAuthzSign(t, privateKey, testAuthzKid, map[string]any{
		"iss":           testAuthzIssuer,
		"sub":           "user-1",
		"subject_type":  constant.SubjectTypeUser,
		"invoke_app_id": "user-app",
		"target_app_id": "svc-b",
		"user_context": map[string]any{
			"user_id": "user-1",
			"app_id":  "user-app",
		},
		"target_service_app_id": "svc-b",
		"route_method":          constant.RequestMethodGrpcString,
		"route_path":            "/acme.test.v1.TestService/Get",
		"decision":              testAuthzDecisionAllow,
		"decision_id":           "decision-1",
		"iat":                   now.Unix(),
		"exp":                   now.Add(time.Minute).Unix(),
	})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.AuthzSign, raw,
	))
	_, err = BuildVerifiedContext(ctx, BuildContextOptions{
		ServiceAppId: "svc-a",
		AuthzVerification: &AuthzSignVerificationOptions{
			PublicKeys: map[string]ed25519.PublicKey{testAuthzKid: publicKey},
			Issuer:     testAuthzIssuer,
			Now:        func() time.Time { return now },
		},
	})
	if !errors.Is(err, ErrAuthzSignInvalidClaims) {
		t.Fatalf("expected target app mismatch to be rejected, got %v", err)
	}
}

func TestBuildVerifiedContext_RequiresServiceAppIdWhenVerificationEnabled(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	now := time.Unix(1710000000, 0).UTC()
	raw := signTestAuthzSign(t, privateKey, testAuthzKid, map[string]any{
		"iss":           testAuthzIssuer,
		"sub":           "user-1",
		"subject_type":  constant.SubjectTypeUser,
		"invoke_app_id": "user-app",
		"target_app_id": "svc-a",
		"user_context": map[string]any{
			"user_id": "user-1",
			"app_id":  "user-app",
		},
		"target_service_app_id": "svc-a",
		"route_method":          constant.RequestMethodGrpcString,
		"route_path":            "/acme.test.v1.TestService/Get",
		"decision":              testAuthzDecisionAllow,
		"decision_id":           "decision-1",
		"iat":                   now.Unix(),
		"exp":                   now.Add(time.Minute).Unix(),
	})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.AuthzSign, raw,
	))
	_, err = BuildVerifiedContext(ctx, BuildContextOptions{
		AuthzVerification: &AuthzSignVerificationOptions{
			PublicKeys: map[string]ed25519.PublicKey{testAuthzKid: publicKey},
			Issuer:     testAuthzIssuer,
			Now:        func() time.Time { return now },
		},
	})
	if !errors.Is(err, ErrAuthzSignInvalidClaims) {
		t.Fatalf("expected missing local service app id to be rejected, got %v", err)
	}
}

func TestBuildContext_UsesTraceSpan(t *testing.T) {
	provider := trace.NewTracerProvider()
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer("service-test")
	ctx, span := tracer.Start(context.Background(), "build-context")
	defer span.End()

	value := BuildContext(ctx, BuildContextOptions{})
	if value.TraceId == "" {
		t.Fatal("expected trace id to be populated from active span")
	}
}

func TestWithContextAndFromContext(t *testing.T) {
	base := context.Background()
	withValue := WithContext(base, &Context{UserId: "user-1"})

	value, ok := FromContext(withValue)
	if !ok {
		t.Fatal("expected service context in context")
	}
	if value.UserId != "user-1" {
		t.Fatalf("unexpected user id: %+v", value)
	}
}
