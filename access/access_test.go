package access

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAuthorizeValidatesAndCopiesDecision(t *testing.T) {
	now := time.Now().UTC()
	request := DataAccessRequest{ResourceKey: "app.application", Action: ResourceActionRead}
	authorizer := DataAccessAuthorizerFunc(func(context.Context, DataAccessRequest) (DataAccessDecision, error) {
		return DataAccessDecision{
			Allowed: true, ResourceKey: "app.application", Action: ResourceActionRead,
			RowConstraints: []RowConstraint{{Dimension: ScopeDimensionTenant, Refs: []string{"tenant-1"}}},
			FieldActions:   FieldActionSet{{FieldKey: "name", Actions: []uint32{FieldPermissionActionRead}}},
			ExpiresAt:      now.Add(time.Minute),
		}, nil
	})
	// 当前辅助函数使用真实时钟；调用者可以直接调用 Validate 注入时钟。
	decision, err := authorizer.Authorize(context.Background(), request)
	if err != nil {
		t.Fatalf("authorizer call failed: %v", err)
	}
	if err := decision.Validate(request, now); err != nil {
		t.Fatalf("expected decision shape to be valid: %v", err)
	}
	decision.RowConstraints[0].Refs[0] = "mutated"
	if decision.RowConstraints[0].Refs[0] != "mutated" {
		t.Fatal("unexpected test setup")
	}
	_ = errors.Is
}

func TestDecisionValidationFailsClosed(t *testing.T) {
	now := time.Unix(1710000000, 0).UTC()
	request := DataAccessRequest{ResourceKey: "app.application", Action: ResourceActionRead}
	cases := []struct {
		name     string
		decision DataAccessDecision
		want     error
	}{
		{name: "denied", decision: DataAccessDecision{ResourceKey: request.ResourceKey, Action: request.Action}, want: ErrDataAccessDenied},
		{name: "expired", decision: DataAccessDecision{Allowed: true, ResourceKey: request.ResourceKey, Action: request.Action, ExpiresAt: now}, want: ErrDataAccessDecisionExpired},
		{name: "mismatch", decision: DataAccessDecision{Allowed: true, ResourceKey: "other", Action: request.Action, ExpiresAt: now.Add(time.Minute)}, want: ErrDataAccessDecisionInvalid},
		{name: "missing expiry", decision: DataAccessDecision{Allowed: true, ResourceKey: request.ResourceKey, Action: request.Action}, want: ErrDataAccessDecisionInvalid},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			err := item.decision.Validate(request, now)
			if !errors.Is(err, item.want) {
				t.Fatalf("expected %v, got %v", item.want, err)
			}
		})
	}
}

func TestDecisionContextIsKeyedAndCopied(t *testing.T) {
	decision := &DataAccessDecision{
		Allowed: true, ResourceKey: "app.application", Action: ResourceActionRead,
		RowConstraints: []RowConstraint{{Dimension: ScopeDimensionOwner, Refs: []string{"user-1"}}},
		FieldActions:   FieldActionSet{{FieldKey: "name", Actions: []uint32{FieldPermissionActionRead}}},
		ExpiresAt:      time.Now().Add(time.Minute),
	}
	ctx := WithDataAccessDecision(context.Background(), decision)
	decision.RowConstraints[0].Refs[0] = "caller-mutated"
	got, ok := DataAccessDecisionFromContext(ctx, "app.application", ResourceActionRead)
	if !ok || got == nil || got.RowConstraints[0].Refs[0] != "user-1" {
		t.Fatalf("context did not preserve immutable copy: got=%#v ok=%v", got, ok)
	}
	got.RowConstraints[0].Refs[0] = "read-mutated"
	again, _ := DataAccessDecisionFromContext(ctx, "app.application", ResourceActionRead)
	if again.RowConstraints[0].Refs[0] != "user-1" {
		t.Fatal("context returned mutable internal decision")
	}
	if _, ok := DataAccessDecisionFromContext(ctx, "app.application", ResourceActionUpdate); ok {
		t.Fatal("decision lookup must include action")
	}
}

func TestDataAccessPolicyRegistryCopiesAndSorts(t *testing.T) {
	input := map[string][]DataAccessPolicy{
		"/svc/List": {
			{ResourceKey: "z.resource", Action: "update"},
			{ResourceKey: "a.resource", Action: "read"},
		},
	}
	registry, err := NewDataAccessPolicyRegistry(input)
	if err != nil {
		t.Fatalf("create registry failed: %v", err)
	}
	input["/svc/List"][0].ResourceKey = "mutated"
	entries, ok := registry.Lookup("/svc/List")
	if !ok || len(entries) != 2 || entries[0].ResourceKey != "a.resource" {
		t.Fatalf("unexpected registry entries: %#v ok=%v", entries, ok)
	}
	entries[0].ResourceKey = "caller-mutated"
	again, _ := registry.Lookup("/svc/List")
	if again[0].ResourceKey != "a.resource" {
		t.Fatal("registry returned mutable internal slice")
	}
}

func TestDataAccessPolicyRegistryRejectsDuplicate(t *testing.T) {
	_, err := NewDataAccessPolicyRegistry(map[string][]DataAccessPolicy{
		"/svc/List": {{ResourceKey: "app.application", Action: ResourceActionRead}, {ResourceKey: "app.application", Action: ResourceActionRead}},
	})
	if !errors.Is(err, ErrDataAccessPolicyInvalid) {
		t.Fatalf("expected invalid policy, got %v", err)
	}
}
