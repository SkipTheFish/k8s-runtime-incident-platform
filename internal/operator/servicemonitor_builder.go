package operator

import (
	obsv1 "observability-platform/api/v1"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BuildServiceMonitor(t *obsv1.ObservabilityTarget) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-" + t.Name,
			Namespace: t.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "observability-platform",
				"observability.io/target":      t.Name,
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: t.Spec.Selector,
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{t.Spec.Namespace},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:     t.Spec.PortName,
					Path:     t.Spec.ScrapePath,
					Interval: monitoringv1.Duration(t.Spec.ScrapeInterval),
				},
			},
		},
	}
}