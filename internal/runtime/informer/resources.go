package informer

import (
	"fmt"
	"time"

	"observability-platform/internal/resourcegraph"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
)

type resourceRegistration struct {
	informer   cache.SharedIndexInformer
	apiVersion string
	kind       string
}

const (
	IndexResourceUID  = "resourceGraphUID"
	IndexResourceName = "resourceGraphName"
)

func ResourceIndexers() cache.Indexers {
	return cache.Indexers{
		IndexResourceUID: func(obj any) ([]string, error) {
			accessor, err := meta.Accessor(obj)
			if err != nil || accessor.GetUID() == "" {
				return nil, nil
			}
			return []string{string(accessor.GetUID())}, nil
		},
		IndexResourceName: func(obj any) ([]string, error) {
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return nil, nil
			}
			return []string{accessor.GetNamespace() + "\x00" + accessor.GetName()}, nil
		},
	}
}

func (i *K8sEventInformer) registerResourceHandler(registration resourceRegistration) error {
	if err := registration.informer.AddIndexers(ResourceIndexers()); err != nil {
		return fmt.Errorf("register %s resource indexes: %w", registration.kind, err)
	}
	handle := func(action resourcegraph.TaskAction, obj any) {
		task, ok := resourceTaskFromObject(i.clusterID, registration.apiVersion, registration.kind, action, obj, time.Now())
		if ok {
			_ = i.resourceEnqueue(task)
		}
	}
	_, err := registration.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { handle(resourcegraph.TaskUpsert, obj) },
		UpdateFunc: func(_, current any) { handle(resourcegraph.TaskUpsert, current) },
		DeleteFunc: func(obj any) { handle(resourcegraph.TaskDelete, obj) },
	})
	if err != nil {
		return fmt.Errorf("register %s resource handler: %w", registration.kind, err)
	}
	return nil
}

func resourceTaskFromObject(clusterID, apiVersion, kind string, action resourcegraph.TaskAction, obj any, observedAt time.Time) (resourcegraph.ResourceTask, bool) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	accessor, err := meta.Accessor(obj)
	if err != nil || accessor.GetUID() == "" {
		return resourcegraph.ResourceTask{}, false
	}
	owners := accessor.GetOwnerReferences()
	ownerUIDs := make([]string, 0, len(owners))
	for _, owner := range owners {
		if owner.UID != "" {
			ownerUIDs = append(ownerUIDs, string(owner.UID))
		}
	}
	task := resourcegraph.ResourceTask{
		Action: action,
		Node: resourcegraph.ResourceNode{
			Key:    resourcegraph.ResourceKey{ClusterID: clusterID, APIVersion: apiVersion, Kind: kind, Namespace: accessor.GetNamespace(), Name: accessor.GetName(), UID: string(accessor.GetUID())},
			Labels: accessor.GetLabels(), ObservedAt: observedAt,
		},
		OwnerUIDs: ownerUIDs,
	}
	if pod, ok := obj.(*corev1.Pod); ok {
		task.NodeName = pod.Spec.NodeName
	}
	return task, true
}
