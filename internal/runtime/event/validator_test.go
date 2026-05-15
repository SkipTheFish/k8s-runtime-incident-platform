package event

import (
	"errors"
	"testing"
	"time"
)

func TestValidatorValidEvent(t *testing.T) {
	v := NewDefaultValidator()

	err := v.Validate(RuntimeEvent{
		Type:       TypeAlertTriggered,
		Source:     "prometheus",
		TenantID:   "t1",
		Namespace:  "monitoring",
		PolicyName: "always-firing",
		AlertName:  "AlwaysFiring",
		OccurredAt: time.Now(),
	})

	if err != nil {
		t.Fatalf("expected valid event, got %v", err)
	}
}

func TestValidatorMissingOccurredAt(t *testing.T) {
	v := NewDefaultValidator()

	err := v.Validate(RuntimeEvent{
		Type:      TypeK8sWarning,
		Source:    "kubernetes",
		TenantID:  "t1",
		Namespace: "monitoring",
	})

	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent, got %v", err)
	}
}

func TestValidatorAlertMissingAlertName(t *testing.T) {
	v := NewDefaultValidator()

	err := v.Validate(RuntimeEvent{
		Type:       TypeAlertTriggered,
		Source:     "prometheus",
		TenantID:   "t1",
		Namespace:  "monitoring",
		PolicyName: "always-firing",
		OccurredAt: time.Now(),
	})

	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent, got %v", err)
	}
}