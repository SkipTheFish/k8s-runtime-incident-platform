package operator

import (
	"fmt"
	"strconv"
	// "log"

	obsv1 "observability-platform/api/v1"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func BuildPrometheusRule(p *obsv1.AlertPolicy) *monitoringv1.PrometheusRule {
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "observability-platform",
		"observability.io/alertpolicy": p.Name,
		"observability.io/tenant":      p.Spec.TenantID,
	}

	for k, v := range p.Spec.Labels {
		labels[k] = v
	}

	// 必须最后写，防止用户 labels 覆盖
	labels["release"] = "kube-prom"

	annotations := map[string]string{
		"summary":        p.Spec.PolicyName,
		"description":    "Generated from AlertPolicy " + p.Name,
		"runbook_url":    p.Spec.RunbookURL,
		"owner":          p.Spec.Owner,
		"evaluation_sec": strconv.Itoa(p.Spec.EvaluationSec),
	}

	for k, v := range p.Spec.Annotations {
		annotations[k] = v
	}

	fullExpr := fmt.Sprintf(
		"(%s) %s %v",
		p.Spec.Query,
		p.Spec.Operator,
		p.Spec.Threshold,
	)

	duration := monitoringv1.Duration(p.Spec.Duration)

	// log.Printf("DEBUG rule labels: %+v", labels)

	return &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alertpolicy-" + p.Name,
			Namespace: p.Namespace,
			Labels:    labels,
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name: "alertpolicy-" + p.Name,
					Rules: []monitoringv1.Rule{
						{
							Alert: p.Spec.PolicyName,
							Expr:  intstr.FromString(fullExpr),
							For:   &duration,
							Labels: map[string]string{
								"tenant":       p.Spec.TenantID,
								"severity":     string(p.Spec.Severity),
								"notification": p.Spec.Notification,
							},
							Annotations: annotations,
						},
					},
				},
			},
		},
	}
}