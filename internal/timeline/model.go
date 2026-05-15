package timeline

import "time"

type EventType string

const (
	EventAlertFired   EventType = "alert_fired"
	EventAlertClosed  EventType = "alert_closed"
	EventLogDetected  EventType = "log_detected"
	EventMetricSpike  EventType = "metric_spike"
)

type TimelineEvent struct {
	ID int64

	IncidentID int64

	Type EventType

	TimelineType string

	Severity string

	Message string

	ResourceKind string

	ResourceName string

	Timestamp time.Time
}