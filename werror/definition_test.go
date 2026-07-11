package werror

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
)

func TestDefinitionNew_FixesIdentity(t *testing.T) {
	definition := Definition{
		Code:    codes.Unauthenticated,
		Domain:  "lhdht.auth",
		Reason:  "TOKEN_INVALID",
		Message: "令牌已失效，请重新登录",
	}

	err := definition.New(
		WithDomain("override"),
		WithReason("OVERRIDE"),
		WithDetail("tokenType", "access"),
	)

	if err.Code() != codes.Unauthenticated {
		t.Fatalf("unexpected code: %v", err.Code())
	}
	if err.Domain() != definition.Domain || err.Reason() != definition.Reason {
		t.Fatalf("definition identity was overridden: domain=%q reason=%q", err.Domain(), err.Reason())
	}
	if err.Message() != definition.Message {
		t.Fatalf("unexpected fallback message: %q", err.Message())
	}
	if got := err.Details()["tokenType"]; got != "access" {
		t.Fatalf("unexpected detail: %q", got)
	}
}

func TestDefinitionWrap_PreservesCause(t *testing.T) {
	cause := errors.New("jwt signature invalid")
	definition := Definition{
		Code:    codes.Unauthenticated,
		Domain:  "lhdht.auth",
		Reason:  "TOKEN_INVALID",
		Message: "令牌已失效，请重新登录",
	}

	err := definition.Wrap(cause)
	if !errors.Is(err, cause) {
		t.Fatal("expected wrapped cause")
	}
}

func TestDefinitionValidate(t *testing.T) {
	tests := []struct {
		name       string
		definition Definition
	}{
		{
			name: "missing domain",
			definition: Definition{
				Code: codes.Internal, Reason: "INTERNAL_ERROR", Message: "内部错误",
			},
		},
		{
			name: "domain contains whitespace",
			definition: Definition{
				Code: codes.Internal, Domain: "lhdht auth", Reason: "INTERNAL_ERROR", Message: "内部错误",
			},
		},
		{
			name: "invalid reason",
			definition: Definition{
				Code: codes.Internal, Domain: "lhdht.auth", Reason: "token-invalid", Message: "内部错误",
			},
		},
		{
			name: "ok code",
			definition: Definition{
				Code: codes.OK, Domain: "lhdht.auth", Reason: "TOKEN_INVALID", Message: "内部错误",
			},
		},
		{
			name: "missing fallback message",
			definition: Definition{
				Code: codes.Internal, Domain: "lhdht.auth", Reason: "TOKEN_INVALID",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.definition.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCatalog(t *testing.T) {
	tokenInvalid := Definition{
		Code: codes.Unauthenticated, Domain: "lhdht.auth", Reason: "TOKEN_INVALID", Message: "令牌已失效",
	}
	appNotFound := Definition{
		Code: codes.NotFound, Domain: "lhdht.auth", Reason: "APP_NOT_FOUND", Message: "应用不存在",
	}

	catalog, err := NewCatalog(tokenInvalid, appNotFound)
	if err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	got, ok := catalog.Find("lhdht.auth", "APP_NOT_FOUND")
	if !ok || got != appNotFound {
		t.Fatalf("unexpected catalog result: %+v", got)
	}

	definitions := catalog.Definitions()
	definitions[0].Reason = "CHANGED"
	got, ok = catalog.Find("lhdht.auth", "TOKEN_INVALID")
	if !ok || got.Reason != "TOKEN_INVALID" {
		t.Fatalf("catalog must be immutable: %+v", got)
	}
}

func TestNewCatalog_RejectsDuplicateDefinition(t *testing.T) {
	definition := Definition{
		Code: codes.NotFound, Domain: "lhdht.auth", Reason: "APP_NOT_FOUND", Message: "应用不存在",
	}
	if _, err := NewCatalog(definition, definition); err == nil {
		t.Fatal("expected duplicate definition error")
	}
}
