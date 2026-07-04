package invocation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type testDialer struct {
	conn *grpc.ClientConn
	err  error
}

func (d testDialer) Dial(ctx context.Context, service *DNS) (*grpc.ClientConn, error) {
	return d.conn, d.err
}

func (d testDialer) Close() error {
	return nil
}

func TestNewUnaryInvoker_SetsDefaultTimeout(t *testing.T) {
	invoker := NewUnaryInvoker(testDialer{conn: &grpc.ClientConn{}}, 0)
	if invoker == nil {
		t.Fatal("expected invoker")
	}
	if invoker.Timeout != DefaultInvokeTimeout {
		t.Fatalf("unexpected invoker timeout: %s", invoker.Timeout)
	}
}

func TestUnaryInvoker_Invoke_ReusesIncomingMetadataAndInvokeFunc(t *testing.T) {
	invoked := false

	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			invoked = true

			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatalf("expected outgoing metadata")
			}
			if got := md.Get(constant.UserAuthority); len(got) == 0 || got[0] != "user-token" {
				t.Fatalf("unexpected user authority metadata: %v", got)
			}
			if got := md.Get(constant.UserId); len(got) != 0 {
				t.Fatalf("expected stale user id metadata to be removed: %v", got)
			}

			return nil
		},
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserAuthority, "user-token",
		constant.UserId, "u-1",
	))

	err := invoker.Invoke(ctx, &DNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !invoked {
		t.Fatalf("expected invoke func to be called")
	}
}

func TestUnaryInvoker_Invoke_PreservesTraceMetadataAndSpanContext(t *testing.T) {
	traceID := trace.TraceID{0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6, 0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36}
	spanID := trace.SpanID{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatal("expected outgoing metadata")
			}
			if got := md.Get(constant.TraceParent); len(got) == 0 || got[0] != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
				t.Fatalf("expected traceparent to be preserved, got %v", got)
			}
			if got := md.Get(constant.TraceState); len(got) == 0 || got[0] != "vendor=value" {
				t.Fatalf("expected tracestate to be preserved, got %v", got)
			}
			if got := md.Get(constant.Baggage); len(got) == 0 || got[0] != "tenant=demo" {
				t.Fatalf("expected baggage to be preserved, got %v", got)
			}
			if got := trace.SpanContextFromContext(ctx); !got.IsValid() || got.TraceID() != traceID {
				t.Fatalf("expected span context to remain on outgoing ctx, got %v", got)
			}
			return nil
		},
	}

	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
		constant.TraceParent, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		constant.TraceState, "vendor=value",
		constant.Baggage, "tenant=demo",
		constant.UserAuthority, "user-token",
	))

	err := invoker.Invoke(ctx, &DNS{
		Service:   "secure",
		Namespace: "lhdht",
	}, "/acme.secure.code.verify.graphic.v1.SecureGraphicVerifyCodeService/Validate", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUnaryInvoker_Invoke_WithoutExtraOptionsDoesNotPanic(t *testing.T) {
	invoked := false
	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			invoked = true
			return nil
		},
	}

	err := invoker.Invoke(context.Background(), &DNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !invoked {
		t.Fatal("expected invoke func to be called")
	}
}

func TestUnaryInvoker_Invoke_OverridesServiceAuthority(t *testing.T) {
	invoker := &UnaryInvoker{
		Dialer:                   testDialer{conn: &grpc.ClientConn{}},
		ServiceAuthorityProvider: fixedServiceAuthorityProvider("config-service-token"),
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatal("expected outgoing metadata")
			}
			if got := md.Get(constant.UserAuthority); len(got) == 0 || got[0] != "user-token" {
				t.Fatalf("unexpected user authority metadata: %v", got)
			}
			if got := md.Get(constant.UserId); len(got) != 0 {
				t.Fatalf("expected stale user id metadata to be removed: %v", got)
			}
			if got := md.Get(constant.ServiceAuthority); len(got) == 0 || got[0] != "config-service-token" {
				t.Fatalf("unexpected service authority metadata: %v", got)
			}
			return nil
		},
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserAuthority, "user-token",
		constant.UserId, "u-incoming",
	))

	err := invoker.Invoke(ctx, &DNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUnaryInvoker_Invoke_UsesConfiguredTimeout(t *testing.T) {
	invoker := &UnaryInvoker{
		Dialer:  testDialer{conn: &grpc.ClientConn{}},
		Timeout: 200 * time.Millisecond,
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected deadline")
			}
			remaining := time.Until(deadline)
			if remaining <= 0 || remaining > time.Second {
				t.Fatalf("unexpected configured timeout remaining: %s", remaining)
			}
			return nil
		},
	}

	err := invoker.Invoke(context.Background(), &DNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUnaryInvoker_Invoke_UsesDefaultTimeoutWhenUnset(t *testing.T) {
	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected deadline")
			}
			remaining := time.Until(deadline)
			if remaining <= 4*time.Second || remaining > DefaultInvokeTimeout {
				t.Fatalf("unexpected default timeout remaining: %s", remaining)
			}
			return nil
		},
	}

	err := invoker.Invoke(context.Background(), &DNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUnaryInvoker_Invoke_PreservesParentCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: &grpc.ClientConn{}},
		InvokeFunc: func(ctx context.Context, conn *grpc.ClientConn, method string, req any, resp any, options ...grpc.CallOption) error {
			select {
			case <-ctx.Done():
				return nil
			default:
				t.Fatal("expected canceled context")
				return nil
			}
		},
	}

	err := invoker.Invoke(parent, &DNS{
		Service:   "auth",
		Namespace: "default",
	}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestNewOutgoingCallContext_CopiesMetadata(t *testing.T) {
	source := metadata.Pairs("x-request-id", "req-1")
	ctx, cancel := NewOutgoingCallContext(context.Background(), source, 0)
	defer cancel()

	source.Set("x-request-id", "req-2")

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}
	if got := md.Get("x-request-id"); len(got) == 0 || got[0] != "req-1" {
		t.Fatalf("metadata should be copied before writing outgoing context: %v", got)
	}
}

func TestNewOutgoingCallContext_PreservesParentCancellationAndTimeout(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	ctx, cancel := NewOutgoingCallContext(parent, metadata.Pairs("x-request-id", "req-1"), time.Second)
	defer cancel()
	stop()

	select {
	case <-ctx.Done():
	default:
		t.Fatal("expected child context to be canceled with parent")
	}

	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("expected timeout to add deadline")
	}
}

func TestResolveOutgoingMetadata_UsesOutgoingContextMetadataAllowlist(t *testing.T) {
	parent := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
		constant.UserAuthority, "user-token",
		"x-request-id", "req-inherited",
	))
	md, err := resolveOutgoingMetadata(parent, nil)
	if err != nil {
		t.Fatalf("resolve metadata failed: %v", err)
	}

	if got := md.Get(constant.UserAuthority); len(got) == 0 || got[0] != "user-token" {
		t.Fatalf("expected user authority to be preserved, got %v", got)
	}
	if got := md.Get("x-request-id"); len(got) != 0 {
		t.Fatalf("expected non-allowlisted request id to be dropped, got %v", got)
	}
}

func TestResolveOutgoingMetadata_InjectsServiceAuthority(t *testing.T) {
	md, err := resolveOutgoingMetadata(context.Background(), fixedServiceAuthorityProvider("auth-service-token"))
	if err != nil {
		t.Fatalf("resolve metadata failed: %v", err)
	}

	if got := md.Get(constant.ServiceAuthority); len(got) == 0 || got[0] != "auth-service-token" {
		t.Fatalf("unexpected service authority metadata: %v", got)
	}
}

func TestResolveOutgoingMetadata_PreservesUserAuthorityAndAuthzSign(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserAuthority, "user-token",
		constant.ServiceAuthority, "old-service-token",
		"authorization", "foreign-authorization",
		constant.AuthzSign, "old-jws",
		constant.UserId, "user-1",
	))

	md, err := resolveOutgoingMetadata(ctx, fixedServiceAuthorityProvider("new-service-token"))
	if err != nil {
		t.Fatalf("resolve metadata failed: %v", err)
	}
	if got := md.Get(constant.UserAuthority); len(got) == 0 || got[0] != "user-token" {
		t.Fatalf("expected user authority to be preserved, got %v", got)
	}
	if got := md.Get(constant.ServiceAuthority); len(got) == 0 || got[0] != "new-service-token" {
		t.Fatalf("expected service authority to be overridden, got %v", got)
	}
	if got := md.Get("authorization"); len(got) != 0 {
		t.Fatalf("expected authorization to be dropped, got %v", got)
	}
	if got := md.Get(constant.AuthzSign); len(got) == 0 || got[0] != "old-jws" {
		t.Fatalf("expected authz sign to be preserved for downstream authz reuse, got %v", got)
	}
	if got := md.Get(constant.UserId); len(got) != 0 {
		t.Fatalf("expected stale user context header to be dropped, got %v", got)
	}
}

func TestUnaryInvoker_Invoke_ReturnsServiceAuthorityProviderError(t *testing.T) {
	expectedErr := errors.New("service token unavailable")
	invoker := &UnaryInvoker{
		Dialer:                   testDialer{conn: &grpc.ClientConn{}},
		ServiceAuthorityProvider: failingServiceAuthorityProvider{err: expectedErr},
	}

	err := invoker.Invoke(context.Background(), &DNS{Service: "auth", Namespace: "default"}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestUnaryInvoker_Invoke_ReturnsErrorWhenDialerMissing(t *testing.T) {
	var invoker *UnaryInvoker

	err := invoker.Invoke(context.Background(), &DNS{Service: "auth", Namespace: "default"}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err != ErrInvokerDialerIsNil {
		t.Fatalf("expected %v, got %v", ErrInvokerDialerIsNil, err)
	}
}

func TestUnaryInvoker_Invoke_ReturnsErrorWhenMethodEmpty(t *testing.T) {
	invoker := &UnaryInvoker{Dialer: testDialer{conn: &grpc.ClientConn{}}}

	err := invoker.Invoke(context.Background(), &DNS{Service: "auth", Namespace: "default"}, "", struct{}{}, &struct{}{})
	if err != ErrInvokeMethodEmpty {
		t.Fatalf("expected %v, got %v", ErrInvokeMethodEmpty, err)
	}
}

func TestUnaryInvoker_Invoke_PropagatesDialError(t *testing.T) {
	expectedErr := errors.New("dial failed")
	invoker := &UnaryInvoker{
		Dialer: testDialer{err: expectedErr},
	}

	err := invoker.Invoke(context.Background(), &DNS{Service: "auth", Namespace: "default"}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestUnaryInvoker_Invoke_UsesDefaultUnaryInvokeFuncWhenInvokeFuncMissing(t *testing.T) {
	conn, err := grpc.NewClient("passthrough:///auth.default.svc.cluster.local:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	defer func() { _ = conn.Close() }()

	invoker := &UnaryInvoker{
		Dialer: testDialer{conn: conn},
	}

	err = invoker.Invoke(context.Background(), &DNS{Service: "auth", Namespace: "default"}, "/acme.auth.v1.AuthService/Check", struct{}{}, &struct{}{})
	if err == nil {
		t.Fatal("expected error from default grpc invoke path")
	}
}

func TestNewOutgoingCallContext_AllowsNilParentAndNilMetadata(t *testing.T) {
	ctx, cancel := NewOutgoingCallContext(nil, nil, 0)
	defer cancel()

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}
	if len(md) != 0 {
		t.Fatalf("expected empty metadata, got %v", md)
	}
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("expected default timeout deadline")
	}
}

type failingServiceAuthorityProvider struct {
	err error
}

func (p failingServiceAuthorityProvider) ServiceAuthority(context.Context) (string, error) {
	return "", p.err
}

type fixedServiceAuthorityProvider string

func (p fixedServiceAuthorityProvider) ServiceAuthority(context.Context) (string, error) {
	return string(p), nil
}
