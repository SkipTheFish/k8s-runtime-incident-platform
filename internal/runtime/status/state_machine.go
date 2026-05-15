package status

import (
	obsv1 "observability-platform/api/v1"
)

func ResolveState(
	activeAlerts int,
) obsv1.AlertRuntimeState {

	switch {

	case activeAlerts > 0:
		return obsv1.AlertStateFiring

	default:
		return obsv1.AlertStateInactive
	}
}