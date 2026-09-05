package resourcegraph

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProcessorConsumesTaskAndStops(t *testing.T) {
	seen := make(chan ResourceTask, 1)
	processor := NewProcessor(1, 1, func(_ context.Context, task ResourceTask) error { seen <- task; return nil })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- processor.Run(ctx) }()
	want := ResourceTask{Action: TaskUpsert, Node: ResourceNode{Key: ResourceKey{UID: "u"}}}
	if err := processor.Enqueue(want); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-seen:
		if got.Node.Key.UID != "u" {
			t.Fatalf("task=%#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("task was not consumed")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("run error=%v", err)
	}
}

func TestProcessorReturnsApplyError(t *testing.T) {
	want := errors.New("database unavailable")
	processor := NewProcessor(1, 1, func(context.Context, ResourceTask) error { return want })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := processor.Enqueue(ResourceTask{}); err != nil {
		t.Fatal(err)
	}
	if err := processor.Run(ctx); !errors.Is(err, want) {
		t.Fatalf("run error=%v", err)
	}
}
