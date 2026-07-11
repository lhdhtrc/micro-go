package werror

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestError_GRPCStatus(t *testing.T) {
	err := InvalidArgument(
		"验证码已过期/不存在",
		WithDomain("lhdht.secure"),
		WithReason("VERIFY_CODE_EXPIRED"),
		WithDetail("verifyType", "email"),
	)

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected %v, got %v", codes.InvalidArgument, st.Code())
	}
	if st.Message() != "验证码已过期/不存在" {
		t.Fatalf("unexpected message: %q", st.Message())
	}
	details := st.Details()
	if len(details) != 1 {
		t.Fatalf("expected 1 status detail, got %d", len(details))
	}
	info, ok := details[0].(*errdetails.ErrorInfo)
	if !ok {
		t.Fatalf("expected ErrorInfo, got %T", details[0])
	}
	if info.GetDomain() != "lhdht.secure" || info.GetReason() != "VERIFY_CODE_EXPIRED" {
		t.Fatalf("unexpected error identity: domain=%q reason=%q", info.GetDomain(), info.GetReason())
	}
	if got := info.GetMetadata()["verifyType"]; got != "email" {
		t.Fatalf("unexpected metadata: %q", got)
	}
}

func TestWrap_PreservesCause(t *testing.T) {
	cause := errors.New("redis: nil")
	err := Wrap(codes.InvalidArgument, cause, "验证码已过期/不存在")

	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped cause")
	}
	if err.Message() != "验证码已过期/不存在" {
		t.Fatalf("unexpected message: %q", err.Message())
	}
}

func TestFromError_FindsWrappedError(t *testing.T) {
	typedErr := NotFound("资源不存在", WithReason("RESOURCE_NOT_FOUND"))
	wrapped := fmt.Errorf("query user: %w", typedErr)

	got, ok := FromError(wrapped)
	if !ok {
		t.Fatalf("expected wrapped error")
	}
	if got.Code() != codes.NotFound {
		t.Fatalf("expected %v, got %v", codes.NotFound, got.Code())
	}
}

func TestError_IsMatchesReasonAndCode(t *testing.T) {
	sentinel := InvalidArgument("验证码错误", WithDomain("lhdht.secure"), WithReason("VERIFY_CODE_INVALID"))
	err := InvalidArgument("图形验证码错误", WithDomain("lhdht.secure"), WithReason("VERIFY_CODE_INVALID"))

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected errors.Is to match by reason and code")
	}
	if errors.Is(PermissionDenied("验证码错误", WithDomain("lhdht.secure"), WithReason("VERIFY_CODE_INVALID")), sentinel) {
		t.Fatalf("different code must not match sentinel")
	}
	if errors.Is(InvalidArgument("验证码错误", WithDomain("lhdht.auth"), WithReason("VERIFY_CODE_INVALID")), sentinel) {
		t.Fatalf("different domain must not match sentinel")
	}
}

func TestDetailsReturnsCopy(t *testing.T) {
	err := FailedPrecondition("状态不允许", WithDetail("state", "disabled"))

	details := err.Details()
	details["state"] = "enabled"

	if got := err.Details()["state"]; got != "disabled" {
		t.Fatalf("expected immutable details copy, got %q", got)
	}
}

func TestCodeOf(t *testing.T) {
	if got := CodeOf(Unauthenticated("token is required")); got != codes.Unauthenticated {
		t.Fatalf("expected %v, got %v", codes.Unauthenticated, got)
	}
	if got := CodeOf(errors.New("plain")); got != codes.Unknown {
		t.Fatalf("expected %v, got %v", codes.Unknown, got)
	}
}

func TestErrorInfoOf_StatusError(t *testing.T) {
	err := NotFound(
		"用户不存在",
		WithDomain("lhdht.user"),
		WithReason("USER_NOT_FOUND"),
		WithDetail("userId", "user-1"),
	)

	info, ok := ErrorInfoOf(err.GRPCStatus().Err())
	if !ok {
		t.Fatal("expected ErrorInfo")
	}
	if info.GetDomain() != "lhdht.user" || info.GetReason() != "USER_NOT_FOUND" {
		t.Fatalf("unexpected ErrorInfo: %+v", info)
	}
	info.Metadata["userId"] = "changed"

	again, ok := ErrorInfoOf(err)
	if !ok || again.GetMetadata()["userId"] != "user-1" {
		t.Fatalf("expected immutable ErrorInfo metadata, got %+v", again)
	}
}

func TestErrorInfoOf_WithoutStructuredIdentity(t *testing.T) {
	if _, ok := ErrorInfoOf(Internal("internal error", WithDetail("operation", "query"))); ok {
		t.Fatal("plain status error must not expose ErrorInfo")
	}
}
