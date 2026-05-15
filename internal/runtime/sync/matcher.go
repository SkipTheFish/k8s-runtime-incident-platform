package sync

import (
	obsv1 "observability-platform/api/v1"

	runtimeprom "observability-platform/internal/runtime/prometheus"
)

func MatchAlert(
	policy *obsv1.AlertPolicy,
	alert runtimeprom.Alert,
) bool {

	alertName := alert.Labels["alertname"]

	if alertName == "" {
		return false
	}

	return alertName == policy.Spec.PolicyName
}