package incidentfsm

import (
	"testing"
	"time"
)

func TestStatusConstants(t *testing.T) {

	if StatusOpen != "open" {
		t.Fatal("invalid open status")
	}

	if StatusResolved != "resolved" {
		t.Fatal("invalid resolved status")
	}

	if StatusReopened != "reopened" {
		t.Fatal("invalid reopened status")
	}
}

func TestFlappingWindow(t *testing.T) {

	if FlappingWindow != 2*time.Minute {
		t.Fatal("unexpected flapping window")
	}
}