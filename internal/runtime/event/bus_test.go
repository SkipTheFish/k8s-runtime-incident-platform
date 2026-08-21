package event

import (
	"context"
	"errors"
	"testing"
	"time"
)

func validAlertEvent() RuntimeEvent {
	return RuntimeEvent{
		Type:       TypeAlertTriggered,
		Source:     "prometheus",
		TenantID:   "t1",
		Namespace:  "monitoring",
		PolicyName: "always-firing",
		AlertName:  "AlwaysFiring",
		Severity:   "warning",
		Message:    "alert firing",
		OccurredAt: time.Now(),
	}
}

func validK8sWarningEvent() RuntimeEvent {
	return RuntimeEvent{
		Type:      TypeK8sWarning,
		Source:    "kubernetes",
		TenantID:  "t1",
		Namespace: "monitoring",
		Resource: RuntimeResource{
			Kind:      "Pod",
			Name:      "bad-image",
			Namespace: "monitoring",
		},
		Message:    "Back-off pulling image",
		OccurredAt: time.Now(),
	}
}

func TestBusPublishSuccess(t *testing.T) {
	bus := NewBus()
	ch := bus.Subscribe(1)

	err := bus.Publish(context.Background(), validAlertEvent())
	if err != nil {
		t.Fatalf("expected publish success, got %v", err)
	}

	select {
	case evt := <-ch:
		if evt.Type != TypeAlertTriggered {
			t.Fatalf("expected %s, got %s", TypeAlertTriggered, evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("expected event received")
	}

	status := bus.Status()
	if status.Metrics["published"] != 1 {
		t.Fatalf("expected published=1, got %d", status.Metrics["published"])
	}
}

func TestBusValidationFailedEmptySource(t *testing.T) {
	bus := NewBus()
	ch := bus.Subscribe(1)

	evt := validAlertEvent()
	evt.Source = ""

	err := bus.Publish(context.Background(), evt)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent, got %v", err)
	}

	select {
	case <-ch:
		t.Fatal("invalid event should not be delivered")
	default:
	}

	status := bus.Status()
	if status.Metrics["validationFailed"] != 1 {
		t.Fatalf("expected validationFailed=1, got %d", status.Metrics["validationFailed"])
	}
}

func TestBusValidationFailedEmptyNamespace(t *testing.T) {
	bus := NewBus()

	evt := validAlertEvent()
	evt.Namespace = ""

	err := bus.Publish(context.Background(), evt)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent, got %v", err)
	}
}

func TestBusValidationFailedAlertWithoutPolicyName(t *testing.T) {
	bus := NewBus()

	evt := validAlertEvent()
	evt.PolicyName = ""

	err := bus.Publish(context.Background(), evt)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent, got %v", err)
	}
}

func TestBusDedupK8sWarning(t *testing.T) {
	bus := NewBus()
	ch := bus.Subscribe(10)

	evt := validK8sWarningEvent()

	err := bus.Publish(context.Background(), evt)
	if err != nil {
		t.Fatalf("first publish should succeed, got %v", err)
	}

	err = bus.Publish(context.Background(), evt)
	if !errors.Is(err, ErrDuplicated) {
		t.Fatalf("expected ErrDuplicated, got %v", err)
	}

	received := 0
	for {
		select {
		case <-ch:
			received++
		default:
			goto done
		}
	}

done:
	if received != 1 {
		t.Fatalf("expected 1 delivered event, got %d", received)
	}

	status := bus.Status()
	if status.Metrics["deduped"] != 1 {
		t.Fatalf("expected deduped=1, got %d", status.Metrics["deduped"])
	}
}

func TestBusRateLimitK8sWarning(t *testing.T) {
	bus := NewBus()
	ch := bus.Subscribe(100)

	base := validK8sWarningEvent()

	for i := 0; i < 10; i++ {
		evt := base
		evt.Message = "warning-" + time.Now().String() + string(rune(i))

		err := bus.Publish(context.Background(), evt)
		if err != nil {
			t.Fatalf("expected publish %d success, got %v", i, err)
		}
	}

	evt := base
	evt.Message = "warning-over-limit"

	err := bus.Publish(context.Background(), evt)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}

	received := 0
	for {
		select {
		case <-ch:
			received++
		default:
			goto done
		}
	}

done:
	if received != 10 {
		t.Fatalf("expected 10 delivered events, got %d", received)
	}

	status := bus.Status()
	if status.Metrics["rateLimited"] != 1 {
		t.Fatalf("expected rateLimited=1, got %d", status.Metrics["rateLimited"])
	}
}

func TestBusDropWhenSubscriberFull(t *testing.T) {
	bus := NewBus()
	_ = bus.Subscribe(0)

	err := bus.Publish(context.Background(), validAlertEvent())
	if err != nil {
		t.Fatalf("expected publish success, got %v", err)
	}

	status := bus.Status()
	if status.Metrics["dropped"] != 1 {
		t.Fatalf("expected dropped=1, got %d", status.Metrics["dropped"])
	}
}

func TestBusMultipleSubscribers(t *testing.T) {
	bus := NewBus()

	ch1 := bus.Subscribe(1)
	ch2 := bus.Subscribe(1)

	err := bus.Publish(context.Background(), validAlertEvent())
	if err != nil {
		t.Fatalf("expected publish success, got %v", err)
	}

	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("subscriber 1 did not receive event")
	}

	select {
	case <-ch2:
	case <-time.After(time.Second):
		t.Fatal("subscriber 2 did not receive event")
	}

	status := bus.Status()
	if status.Subscribers != 2 {
		t.Fatalf("expected subscribers=2, got %d", status.Subscribers)
	}
}
