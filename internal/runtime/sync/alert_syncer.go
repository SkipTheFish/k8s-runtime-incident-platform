package sync

import (
	"context"
	"time"
	"log"
	"fmt"

	obsv1 "observability-platform/api/v1"
	runtimeprom "observability-platform/internal/runtime/prometheus"
	runtimestatus "observability-platform/internal/runtime/status"
	runtimeevent "observability-platform/internal/runtime/event"
	runtimeResource "observability-platform/internal/runtime/resource"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AlertSyncer struct {
	client client.Client

	promClient *runtimeprom.Client
	bus        *runtimeevent.Bus
}

func NewAlertSyncer(
	client client.Client,
	promClient *runtimeprom.Client,
	bus *runtimeevent.Bus,
) *AlertSyncer {
	return &AlertSyncer{
		client:     client,
		promClient: promClient,
		bus:        bus,
	}
}

func (s *AlertSyncer) Start(ctx context.Context) {
	// log.Println("SYNC: AlertSyncer started")


	// log.Println("SYNC: first sync now")
	s.sync(ctx)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// log.Println("SYNC: ticker sync")
			s.sync(ctx)

		case <-ctx.Done():
			// log.Println("SYNC: stopped")
			return
		}
	}
}

func (s *AlertSyncer) sync(ctx context.Context) {

	var policies obsv1.AlertPolicyList

	if err := s.client.List(ctx, &policies); err != nil {
		return
	}

	for i := range policies.Items {

		p := &policies.Items[i]

		s.projectPolicy(ctx, p)
	}

	log.Println("SYNC: sync() entered")
}

func (s *AlertSyncer) projectPolicy(
	ctx context.Context,
	policy *obsv1.AlertPolicy,
) {

	value, labels, err := s.promClient.QueryWithLabels(
		ctx,
		policy.Spec.Query,
	)
	if err != nil {
		log.Printf("SYNC: prometheus query failed policy=%s/%s err=%v",
			policy.Namespace,
			policy.Name,
			err,
		)
		return
	}

	active := 0

	if value > policy.Spec.Threshold {
		active = 1
	}

	now := metav1.NewTime(time.Now())

	resource := inferResourceFromLabels(labels)

	if resource.Kind == "" {
		resource = runtimeResource.InferFromQuery(
			policy.Spec.Query,
		)
	}

	oldState := policy.Status.State
	newState := runtimestatus.ResolveState(active)

	if oldState != obsv1.AlertStateFiring &&
		newState == obsv1.AlertStateFiring {

		policy.Status.LastTriggeredAt = &now
	}

	if oldState == obsv1.AlertStateFiring &&
		newState == obsv1.AlertStateInactive {

		policy.Status.LastResolvedAt = &now
	}

	if policy.Status.ActiveAlerts == active &&
		oldState == newState {
		return
	}

	policy.Status.ActiveAlerts = active
	policy.Status.State = newState
	policy.Status.LastEvaluationTime = &now

	if oldState != newState {
		log.Printf("EVENT BUS: publishing old=%s new=%s policy=%s/%s resource=%s/%s",
			oldState,
			newState,
			policy.Namespace,
			policy.Name,
			resource.Kind,
			resource.Name,
		)

		switch {

		case newState == obsv1.AlertStateFiring:

			evt := runtimeevent.RuntimeEvent{
				Type:       runtimeevent.TypeAlertTriggered,
				Source:     "prometheus",
				TenantID:   policy.Spec.TenantID,
				Namespace:  policy.Namespace,
				PolicyName: policy.Spec.PolicyName,
				AlertName:  policy.Name,
				Severity:   string(policy.Spec.Severity),
				Resource:   resource,
				Message:    "Alert entered firing state",
				OccurredAt: time.Now(),
			}

			if err := s.bus.Publish(ctx, evt); err != nil {
				log.Printf("EVENT BUS: publish failed err=%v evt=%+v", err, evt)
			}

		case oldState == obsv1.AlertStateFiring &&
			newState == obsv1.AlertStateInactive:

			evt := runtimeevent.RuntimeEvent{
				Type:       runtimeevent.TypeAlertResolved,
				Source:     "prometheus",
				TenantID:   policy.Spec.TenantID,
				Namespace:  policy.Namespace,
				PolicyName: policy.Spec.PolicyName,
				AlertName:  policy.Name,
				Severity:   string(policy.Spec.Severity),
				Resource:   resource,
				Message:    "Alert resolved",
				OccurredAt: time.Now(),
			}

			if err := s.bus.Publish(ctx, evt); err != nil {
				log.Printf("EVENT BUS: publish failed err=%v evt=%+v", err, evt)
			}
		}
	}

	if err := s.client.Status().Update(ctx, policy); err != nil {
		fmt.Printf("❌ Status update failed: %v\n", err)
	}
}

func inferResourceFromLabels(
	labels map[string]string,
) runtimeevent.RuntimeResource {

	if pod := labels["pod"]; pod != "" {
		return runtimeevent.RuntimeResource{
			Kind: "Pod",
			Name: pod,
		}
	}

	if pod := labels["pod_name"]; pod != "" {
		return runtimeevent.RuntimeResource{
			Kind: "Pod",
			Name: pod,
		}
	}

	if node := labels["node"]; node != "" {
		return runtimeevent.RuntimeResource{
			Kind: "Node",
			Name: node,
		}
	}

	if deployment := labels["deployment"]; deployment != "" {
		return runtimeevent.RuntimeResource{
			Kind: "Deployment",
			Name: deployment,
		}
	}

	return runtimeevent.RuntimeResource{}
}