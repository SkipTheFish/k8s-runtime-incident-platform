package incident

import (
	"testing"
	"time"
)

func TestTransitionMatrix(t *testing.T) {
	statuses := []IncidentStatus{IncidentOpen, IncidentAcknowledged, IncidentResolved, IncidentReopened}
	allowed := map[[2]IncidentStatus]bool{
		{IncidentOpen, IncidentAcknowledged}:     true,
		{IncidentOpen, IncidentResolved}:         true,
		{IncidentAcknowledged, IncidentResolved}: true,
		{IncidentResolved, IncidentReopened}:     true,
		{IncidentReopened, IncidentAcknowledged}: true,
		{IncidentReopened, IncidentResolved}:     true,
	}
	for _, from := range statuses {
		for _, to := range statuses {
			t.Run(string(from)+"_to_"+string(to), func(t *testing.T) {
				result, err := Transition(from, validTransition(to))
				if from == to {
					if err != nil || result.Changed {
						t.Fatalf("idempotent transition result=%#v err=%v", result, err)
					}
					return
				}
				if allowed[[2]IncidentStatus{from, to}] {
					if err != nil || !result.Changed || result.From != from || result.To != to {
						t.Fatalf("result=%#v err=%v", result, err)
					}
					return
				}
				if !IsCode(err, CodeInvalidTransition) {
					t.Fatalf("error=%v", err)
				}
			})
		}
	}
}

func TestTransitionRequiresAuditInput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*TransitionCommand)
	}{
		{"actor", func(c *TransitionCommand) { c.Actor = "" }},
		{"reason", func(c *TransitionCommand) { c.Reason = "" }},
		{"occurredAt", func(c *TransitionCommand) { c.OccurredAt = time.Time{} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := validTransition(IncidentResolved)
			tt.mutate(&command)
			_, err := Transition(IncidentOpen, command)
			if !IsCode(err, CodeInvalidCommand) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func validTransition(target IncidentStatus) TransitionCommand {
	return TransitionCommand{Target: target, Actor: "operator@example.com", Reason: "investigation complete", OccurredAt: time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)}
}
