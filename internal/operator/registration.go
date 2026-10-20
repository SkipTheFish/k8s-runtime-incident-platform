package operator

import (
	"fmt"

	"observability-platform/internal/correlation"

	ctrl "sigs.k8s.io/controller-runtime"
)

// RegisterControllers is the single production and envtest registration path.
func RegisterControllers(mgr ctrl.Manager, registry *correlation.Registry, repository Repository) error {
	controllers := []struct {
		name  string
		setup func(ctrl.Manager) error
	}{
		{name: "AlertPolicy", setup: (&AlertPolicyReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Repo: repository}).SetupWithManager},
		{name: "CorrelationPolicy", setup: (&CorrelationPolicyReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Registry: registry}).SetupWithManager},
		{name: "ObservabilityTarget", setup: (&ObservabilityTargetReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
		{name: "ObserveTenant", setup: (&TenantReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager},
	}
	for _, controller := range controllers {
		if err := controller.setup(mgr); err != nil {
			return fmt.Errorf("register %s controller: %w", controller.name, err)
		}
	}
	return nil
}
