package event

import (
	"fmt"
	"strings"
)

type Validator interface {
	Validate(evt RuntimeEvent) error
}

type DefaultValidator struct{}

func NewDefaultValidator() *DefaultValidator {
	return &DefaultValidator{}
}

func (v *DefaultValidator) Validate(evt RuntimeEvent) error {

	// =========================================
	// Required Fields
	// =========================================

	if strings.TrimSpace(string(evt.Type)) == "" {
		return fmt.Errorf("%w: empty type", ErrInvalidEvent)
	}

	if strings.TrimSpace(evt.Source) == "" {
		return fmt.Errorf("%w: empty source", ErrInvalidEvent)
	}

	if strings.TrimSpace(evt.Namespace) == "" {
		return fmt.Errorf("%w: empty namespace", ErrInvalidEvent)
	}

	// =========================================
	// Required Time
	// =========================================

	if evt.OccurredAt.IsZero() {
		return fmt.Errorf("%w: empty occurredAt", ErrInvalidEvent)
	}

	// =========================================
	// Alert Events Validation
	// =========================================

	switch evt.Type {

	case TypeAlertTriggered, TypeAlertResolved:

		if strings.TrimSpace(evt.PolicyName) == "" {
			return fmt.Errorf("%w: empty policyName", ErrInvalidEvent)
		}

		if strings.TrimSpace(evt.AlertName) == "" {
			return fmt.Errorf("%w: empty alertName", ErrInvalidEvent)
		}
	}

	return nil
}
