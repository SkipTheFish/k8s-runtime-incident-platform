package informer

import (
	"context"
	"fmt"
	"time"

	"observability-platform/internal/resourcegraph"
	"observability-platform/internal/signal"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type EnqueueFunc func(signal.Signal) error
type ResourceEnqueueFunc func(resourcegraph.ResourceTask) error

type K8sEventInformer struct {
	config          *rest.Config
	clusterID       string
	enqueue         EnqueueFunc
	resourceEnqueue ResourceEnqueueFunc
	resolverReady   func(*resourcegraph.Resolver)
	ready           func(bool)
	resync          time.Duration
}

func NewK8sEventInformer(config *rest.Config, clusterID string, enqueue EnqueueFunc, ready func(bool)) *K8sEventInformer {
	return &K8sEventInformer{config: config, clusterID: clusterID, enqueue: enqueue, ready: ready, resync: 30 * time.Second}
}

func (i *K8sEventInformer) WithResourceEnqueue(enqueue ResourceEnqueueFunc) *K8sEventInformer {
	i.resourceEnqueue = enqueue
	return i
}

func (i *K8sEventInformer) WithResolver(ready func(*resourcegraph.Resolver)) *K8sEventInformer {
	i.resolverReady = ready
	return i
}

func (i *K8sEventInformer) Start(ctx context.Context) error {
	clientset, err := kubernetes.NewForConfig(i.config)
	if err != nil {
		return fmt.Errorf("create Kubernetes client: %w", err)
	}
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, i.resync, informers.WithNamespace(metav1.NamespaceAll))
	inf := factory.Core().V1().Events().Informer()
	if _, err := inf.AddEventHandler(cache.ResourceEventHandlerFuncs{AddFunc: i.handle, UpdateFunc: func(_, current any) { i.handle(current) }, DeleteFunc: i.handle}); err != nil {
		return fmt.Errorf("register event handler: %w", err)
	}
	if i.resourceEnqueue != nil {
		registrations := []resourceRegistration{
			{factory.Core().V1().Pods().Informer(), "v1", "Pod"},
			{factory.Core().V1().Nodes().Informer(), "v1", "Node"},
			{factory.Apps().V1().Deployments().Informer(), appsv1.SchemeGroupVersion.String(), "Deployment"},
			{factory.Apps().V1().ReplicaSets().Informer(), appsv1.SchemeGroupVersion.String(), "ReplicaSet"},
			{factory.Apps().V1().StatefulSets().Informer(), appsv1.SchemeGroupVersion.String(), "StatefulSet"},
			{factory.Apps().V1().DaemonSets().Informer(), appsv1.SchemeGroupVersion.String(), "DaemonSet"},
			{factory.Batch().V1().Jobs().Informer(), batchv1.SchemeGroupVersion.String(), "Job"},
		}
		for _, registration := range registrations {
			if err := i.registerResourceHandler(registration); err != nil {
				return err
			}
		}
		if i.resolverReady != nil {
			indexed := make([]resourcegraph.IndexedResource, 0, len(registrations))
			for _, registration := range registrations {
				indexed = append(indexed, resourcegraph.IndexedResource{Indexer: registration.informer.GetIndexer(), GVK: schema.FromAPIVersionAndKind(registration.apiVersion, registration.kind)})
			}
			i.resolverReady(resourcegraph.NewResolver(indexed, 8))
		}
	}
	factory.Start(ctx.Done())
	status := factory.WaitForCacheSync(ctx.Done())
	synced := len(status) > 0
	for _, ok := range status {
		synced = synced && ok
	}
	if !synced {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("Kubernetes Event cache sync failed")
	}
	i.ready(true)
	defer i.ready(false)
	<-ctx.Done()
	return ctx.Err()
}

func (i *K8sEventInformer) handle(obj any) {
	event, ok := eventFromObject(obj)
	if !ok {
		return
	}
	s, err := signal.FromKubernetesEvent(i.clusterID, event, time.Now())
	if err != nil {
		return
	}
	_ = i.enqueue(s)
}

func eventFromObject(obj any) (*corev1.Event, bool) {
	if event, ok := obj.(*corev1.Event); ok {
		return event, true
	}
	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if !ok {
		return nil, false
	}
	event, ok := tombstone.Obj.(*corev1.Event)
	return event, ok
}
