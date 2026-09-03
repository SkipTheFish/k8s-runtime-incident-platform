package informer

import (
	"context"
	"fmt"
	"time"

	"observability-platform/internal/signal"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type EnqueueFunc func(signal.Signal) error

type K8sEventInformer struct {
	config    *rest.Config
	clusterID string
	enqueue   EnqueueFunc
	ready     func(bool)
	resync    time.Duration
}

func NewK8sEventInformer(config *rest.Config, clusterID string, enqueue EnqueueFunc, ready func(bool)) *K8sEventInformer {
	return &K8sEventInformer{config: config, clusterID: clusterID, enqueue: enqueue, ready: ready, resync: 30 * time.Second}
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
	factory.Start(ctx.Done())
	status := factory.WaitForCacheSync(ctx.Done())
	synced := false
	for _, ok := range status {
		synced = synced || ok
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
