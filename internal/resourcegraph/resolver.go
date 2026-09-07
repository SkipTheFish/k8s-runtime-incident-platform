package resourcegraph

import (
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

const UIDIndex = "resourceGraphUID"

var (
	ErrResourceNotFound = errors.New("resource not found in informer cache")
	ErrOwnerNotFound    = errors.New("owner not found in informer cache")
	ErrOrphanPod        = errors.New("pod has neither a supported owner nor a scheduled node")
	ErrResolutionDepth  = errors.New("resource ownership exceeds resolution depth")
)

type IndexedResource struct {
	Indexer cache.Indexer
	GVK     schema.GroupVersionKind
}

type Resolver struct {
	resources []IndexedResource
	maxDepth  int
	now       func() time.Time
}

func NewResolver(resources []IndexedResource, maxDepth int) *Resolver {
	if maxDepth < 1 {
		maxDepth = 8
	}
	return &Resolver{resources: resources, maxDepth: maxDepth, now: time.Now}
}

func (r *Resolver) Resolve(key ResourceKey) (RootResolutionResult, error) {
	current, currentGVK, err := r.byUID(key.UID)
	if err != nil {
		return RootResolutionResult{}, fmt.Errorf("resolve %s/%s: %w", key.Kind, key.Name, err)
	}
	result := RootResolutionResult{}
	for depth := 0; depth < r.maxDepth; depth++ {
		node, err := r.node(current, currentGVK)
		if err != nil {
			return RootResolutionResult{}, err
		}
		node.Key.ClusterID = key.ClusterID
		if isRootKind(node.Key.Kind) {
			result.Root = node
			return result, nil
		}
		accessor, err := meta.Accessor(current)
		if err != nil {
			return RootResolutionResult{}, fmt.Errorf("access resource metadata: %w", err)
		}
		ownerUID := controllerOwnerUID(accessor.GetOwnerReferences())
		if ownerUID != "" {
			owner, ownerGVK, err := r.byUID(ownerUID)
			if err != nil {
				return RootResolutionResult{}, fmt.Errorf("%w: uid %s", ErrOwnerNotFound, ownerUID)
			}
			result.Path = append(result.Path, ResourceEdge{ClusterID: key.ClusterID, FromUID: node.Key.UID, ToUID: ownerUID, Type: EdgeOwnedBy, ObservedAt: r.now()})
			current, currentGVK = owner, ownerGVK
			continue
		}
		if pod, ok := current.(*corev1.Pod); ok && pod.Spec.NodeName != "" {
			nodeObject, nodeGVK, err := r.nodeByName(pod.Spec.NodeName)
			if err != nil {
				return RootResolutionResult{}, err
			}
			nodeUID := string(nodeObject.(*corev1.Node).UID)
			result.Path = append(result.Path, ResourceEdge{ClusterID: key.ClusterID, FromUID: node.Key.UID, ToUID: nodeUID, Type: EdgeScheduledOn, ObservedAt: r.now()})
			root, err := r.node(nodeObject, nodeGVK)
			if err != nil {
				return RootResolutionResult{}, err
			}
			root.Key.ClusterID = key.ClusterID
			result.Root = root
			return result, nil
		}
		if node.Key.Kind == "Pod" {
			return RootResolutionResult{}, ErrOrphanPod
		}
		return RootResolutionResult{}, fmt.Errorf("unsupported non-root resource kind %s", node.Key.Kind)
	}
	return RootResolutionResult{}, ErrResolutionDepth
}

func (r *Resolver) byUID(uid string) (any, schema.GroupVersionKind, error) {
	for _, resource := range r.resources {
		objects, err := resource.Indexer.ByIndex(UIDIndex, uid)
		if err != nil {
			return nil, schema.GroupVersionKind{}, err
		}
		if len(objects) > 0 {
			return objects[0], resource.GVK, nil
		}
	}
	return nil, schema.GroupVersionKind{}, ErrResourceNotFound
}

func (r *Resolver) nodeByName(name string) (any, schema.GroupVersionKind, error) {
	for _, resource := range r.resources {
		if resource.GVK.Kind != "Node" {
			continue
		}
		objects, err := resource.Indexer.ByIndex("resourceGraphName", "\x00"+name)
		if err != nil {
			return nil, schema.GroupVersionKind{}, err
		}
		if len(objects) > 0 {
			return objects[0], resource.GVK, nil
		}
	}
	return nil, schema.GroupVersionKind{}, fmt.Errorf("%w: Node %s", ErrResourceNotFound, name)
}

func (r *Resolver) node(obj any, gvk schema.GroupVersionKind) (ResourceNode, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return ResourceNode{}, err
	}
	return ResourceNode{Key: ResourceKey{APIVersion: gvk.GroupVersion().String(), Kind: gvk.Kind, Namespace: accessor.GetNamespace(), Name: accessor.GetName(), UID: string(accessor.GetUID())}, Labels: accessor.GetLabels(), ObservedAt: r.now()}, nil
}

func controllerOwnerUID(owners []metav1.OwnerReference) string {
	for _, owner := range owners {
		if owner.Controller != nil && *owner.Controller {
			return string(owner.UID)
		}
	}
	return ""
}

func isRootKind(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "Job", "Node":
		return true
	}
	return false
}
