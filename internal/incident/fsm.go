package incident

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	IncidentAcknowledged IncidentStatus = "acknowledged"
	IncidentReopened     IncidentStatus = "reopened"
)

type ErrorCode string

const (
	CodeInvalidTransition ErrorCode = "INCIDENT_INVALID_TRANSITION"
	CodeInvalidCommand    ErrorCode = "INCIDENT_INVALID_COMMAND"
	CodeVersionConflict   ErrorCode = "INCIDENT_VERSION_CONFLICT"
)

type DomainError struct {
	Code    ErrorCode
	Message string
}

func (e *DomainError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func IsCode(err error, code ErrorCode) bool {
	var domain *DomainError
	return errors.As(err, &domain) && domain.Code == code
}

type TransitionCommand struct {
	Target          IncidentStatus `json:"target"`
	Actor           string         `json:"actor"`
	Reason          string         `json:"reason"`
	OccurredAt      time.Time      `json:"occurredAt"`
	ExpectedVersion *int64         `json:"-"`
}

type TransitionResult struct {
	From    IncidentStatus
	To      IncidentStatus
	Changed bool
}

var allowedTransitions = map[IncidentStatus]map[IncidentStatus]struct{}{
	IncidentOpen:         {IncidentAcknowledged: {}, IncidentResolved: {}},
	IncidentAcknowledged: {IncidentResolved: {}},
	IncidentResolved:     {IncidentReopened: {}},
	IncidentReopened:     {IncidentAcknowledged: {}, IncidentResolved: {}},
}

func Transition(current IncidentStatus, command TransitionCommand) (TransitionResult, error) {
	if strings.TrimSpace(command.Actor) == "" || strings.TrimSpace(command.Reason) == "" || command.OccurredAt.IsZero() {
		return TransitionResult{}, &DomainError{Code: CodeInvalidCommand, Message: "actor, reason, and occurredAt are required"}
	}
	if current == command.Target {
		return TransitionResult{From: current, To: current, Changed: false}, nil
	}
	if _, ok := allowedTransitions[current][command.Target]; !ok {
		return TransitionResult{}, &DomainError{Code: CodeInvalidTransition, Message: fmt.Sprintf("cannot transition from %s to %s", current, command.Target)}
	}
	return TransitionResult{From: current, To: command.Target, Changed: true}, nil
}
