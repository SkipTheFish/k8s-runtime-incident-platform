package informer

import (
	"context"
	"log"
	"time"

	runtimeevent "observability-platform/internal/runtime/event"
	runtimegraph "observability-platform/internal/runtime/graph"

	corev1 "k8s.io/api/core/v1"
	// "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type K8sEventInformer struct {
	config    *rest.Config
	bus       *runtimeevent.Bus
	graphRepo *runtimegraph.Repository
}

func NewK8sEventInformer(
	config *rest.Config,
	bus *runtimeevent.Bus,
	graphRepo *runtimegraph.Repository,
) *K8sEventInformer {
	return &K8sEventInformer{
		config:    config,
		bus:       bus,
		graphRepo: graphRepo,
	}
}

func (i *K8sEventInformer) Start(ctx context.Context) error {
	clientset, err := kubernetes.NewForConfig(i.config)
	if err != nil {
		return err
	}

	factory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		30*time.Second,
		informers.WithNamespace(metav1.NamespaceAll),
	)

	inf := factory.Core().V1().Events().Informer()

	_, err = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			i.handle(ctx, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			i.handle(ctx, newObj)
		},
	})
	if err != nil {
		return err
	}

	log.Println("K8S INFORMER: event informer started")
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	<-ctx.Done()

	log.Println("K8S INFORMER: envent informer stopped")

	return nil
}

func (i *K8sEventInformer) handle(ctx context.Context, obj interface{}) {
	ev, ok := obj.(*corev1.Event)
	if !ok {
		return
	}

	if ev.Type != corev1.EventTypeWarning {
		return
	}

	if !ev.LastTimestamp.IsZero() && time.Since(ev.LastTimestamp.Time) > 2*time.Minute {
		return
	}

	kind := ev.InvolvedObject.Kind
	name := ev.InvolvedObject.Name
	namespace := ev.Namespace

	_ = i.graphRepo.UpsertNode(ctx, kind, namespace, name, "")

	i.bus.Publish(ctx, runtimeevent.RuntimeEvent{
		Type:         runtimeevent.TypeK8sWarning,
		Source:       "kubernetes",
		Namespace:    namespace,
		Resource: runtimeevent.RuntimeResource{
			Kind: kind,
			Name: name,
			Namespace: namespace,
		},
		Message:      ev.Message,
		OccurredAt:   time.Now(),
	})
}