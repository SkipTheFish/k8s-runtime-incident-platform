package timeline

func ToDTO(
	events []TimelineEvent,
) []TimelineEventDTO {

	result := make(
		[]TimelineEventDTO,
		0,
		len(events),
	)

	for _, e := range events {

		result = append(result, TimelineEventDTO{

			ID:         e.ID,

			IncidentID: e.IncidentID,

			EventType: string(e.Type),

			TimelineType: mapTimelineType(EventType(e.Type)),

			Severity: mapSeverity(EventType(e.Type)),

			Message: e.Message,

			ResourceKind: e.ResourceKind,

			ResourceName: e.ResourceName,

			OccurredAt: e.Timestamp,
		})
	}

	return result
}

func mapTimelineType(
	t EventType,
) string {

	switch t {

	case EventAlertFired:
		return "SYMPTOM"

	case EventMetricSpike:
		return "SYMPTOM"

	case EventLogDetected:
		return "CAUSE"

	default:
		return "SYSTEM"
	}
}

func mapSeverity(
	t EventType,
) string {

	switch t {

	case EventAlertFired:
		return "critical"

	case EventMetricSpike:
		return "warning"

	default:
		return "info"
	}
}