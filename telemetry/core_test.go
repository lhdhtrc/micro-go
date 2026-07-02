package telemetry

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestNewResourceIncludesEnvironment(t *testing.T) {
	res, err := newResource(&Resource{
		ServiceId:         "app-id",
		Environment:       "prod",
		ServiceName:       "app",
		ServiceVersion:    "v0.1.0",
		ServiceNamespace:  "lhdht",
		ServiceInstanceId: "instance-id",
	})
	if err != nil {
		t.Fatalf("new resource failed: %v", err)
	}
	if got, ok := res.Set().Value(attribute.Key("deployment.environment")); !ok || got.AsString() != "prod" {
		t.Fatalf("unexpected deployment.environment: got=%s want=%s", got.AsString(), "prod")
	}
}
