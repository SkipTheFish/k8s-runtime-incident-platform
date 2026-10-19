package telemetry

import (
	"context"
	"testing"
)

func TestConfigureWithoutEndpointIsNoop(t *testing.T) {
	shutdown, err := Configure(context.Background(), "", "test")
	if err != nil || shutdown == nil {
		t.Fatalf("shutdown=%v err=%v", shutdown, err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
