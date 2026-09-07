package resourcegraph

import (
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

func TestResolverTopologyPaths(t *testing.T) {
	tests := []struct {
		name     string
		rootKind string
		objects  []objectWithGVK
		wantPath int
	}{
		{"deployment", "Deployment", []objectWithGVK{
			{pod("p", "rs", ""), gvk("v1", "Pod")},
			{&appsv1.ReplicaSet{ObjectMeta: objectMeta("rs", "deploy")}, gvk("apps/v1", "ReplicaSet")},
			{&appsv1.Deployment{ObjectMeta: plainMeta("deploy", "api")}, gvk("apps/v1", "Deployment")},
		}, 2},
		{"statefulset", "StatefulSet", []objectWithGVK{{pod("p", "sts", ""), gvk("v1", "Pod")}, {&appsv1.StatefulSet{ObjectMeta: plainMeta("sts", "db")}, gvk("apps/v1", "StatefulSet")}}, 1},
		{"daemonset", "DaemonSet", []objectWithGVK{{pod("p", "ds", ""), gvk("v1", "Pod")}, {&appsv1.DaemonSet{ObjectMeta: plainMeta("ds", "agent")}, gvk("apps/v1", "DaemonSet")}}, 1},
		{"job", "Job", []objectWithGVK{{pod("p", "job", ""), gvk("v1", "Pod")}, {&batchv1.Job{ObjectMeta: plainMeta("job", "task")}, gvk("batch/v1", "Job")}}, 1},
		{"node", "Node", []objectWithGVK{{pod("p", "", "node-a"), gvk("v1", "Pod")}, {&corev1.Node{ObjectMeta: plainClusterMeta("node", "node-a")}, gvk("v1", "Node")}}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := resolverFromObjects(t, tt.objects)
			got, err := resolver.Resolve(ResourceKey{ClusterID: "c", Kind: "Pod", Name: "pod", UID: "p"})
			if err != nil {
				t.Fatal(err)
			}
			if got.Root.Key.Kind != tt.rootKind || got.Root.Key.ClusterID != "c" || len(got.Path) != tt.wantPath {
				t.Fatalf("result=%#v", got)
			}
		})
	}
}

func TestResolverBrokenOrphanDeletedAndDepth(t *testing.T) {
	t.Run("broken owner", func(t *testing.T) {
		r := resolverFromObjects(t, []objectWithGVK{{pod("p", "missing", ""), gvk("v1", "Pod")}})
		_, err := r.Resolve(ResourceKey{ClusterID: "c", Kind: "Pod", UID: "p"})
		if !errors.Is(err, ErrOwnerNotFound) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("orphan", func(t *testing.T) {
		r := resolverFromObjects(t, []objectWithGVK{{pod("p", "", ""), gvk("v1", "Pod")}})
		_, err := r.Resolve(ResourceKey{ClusterID: "c", Kind: "Pod", UID: "p"})
		if !errors.Is(err, ErrOrphanPod) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("deleted owner absent from cache", func(t *testing.T) {
		r := resolverFromObjects(t, []objectWithGVK{{pod("p", "gone", ""), gvk("v1", "Pod")}})
		_, err := r.Resolve(ResourceKey{ClusterID: "c", Kind: "Pod", UID: "p"})
		if !errors.Is(err, ErrOwnerNotFound) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("bounded cycle", func(t *testing.T) {
		a := &appsv1.ReplicaSet{ObjectMeta: objectMeta("a", "b")}
		b := &appsv1.ReplicaSet{ObjectMeta: objectMeta("b", "a")}
		r := resolverFromObjects(t, []objectWithGVK{{a, gvk("apps/v1", "ReplicaSet")}, {b, gvk("apps/v1", "ReplicaSet")}})
		r.maxDepth = 3
		_, err := r.Resolve(ResourceKey{ClusterID: "c", Kind: "ReplicaSet", UID: "a"})
		if !errors.Is(err, ErrResolutionDepth) {
			t.Fatalf("error=%v", err)
		}
	})
}

type objectWithGVK struct {
	object any
	gvk    schema.GroupVersionKind
}

func resolverFromObjects(t *testing.T, objects []objectWithGVK) *Resolver {
	t.Helper()
	groups := map[schema.GroupVersionKind]cache.Indexer{}
	for _, item := range objects {
		indexer := groups[item.gvk]
		if indexer == nil {
			indexer = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
				UIDIndex: func(obj any) ([]string, error) {
					accessor, _ := meta.Accessor(obj)
					return []string{string(accessor.GetUID())}, nil
				},
				"resourceGraphName": func(obj any) ([]string, error) {
					accessor, _ := meta.Accessor(obj)
					return []string{accessor.GetNamespace() + "\x00" + accessor.GetName()}, nil
				},
			})
			groups[item.gvk] = indexer
		}
		if err := indexer.Add(item.object); err != nil {
			t.Fatal(err)
		}
	}
	resources := make([]IndexedResource, 0, len(groups))
	for kind, indexer := range groups {
		resources = append(resources, IndexedResource{Indexer: indexer, GVK: kind})
	}
	return NewResolver(resources, 8)
}

func pod(uid, ownerUID, node string) *corev1.Pod {
	meta := plainMeta(uid, "pod")
	if ownerUID != "" {
		meta.OwnerReferences = []metav1.OwnerReference{{UID: types.UID(ownerUID), Controller: ptr(true)}}
	}
	return &corev1.Pod{ObjectMeta: meta, Spec: corev1.PodSpec{NodeName: node}}
}
func objectMeta(uid, ownerUID string) metav1.ObjectMeta {
	m := plainMeta(uid, uid)
	m.OwnerReferences = []metav1.OwnerReference{{UID: types.UID(ownerUID), Controller: ptr(true)}}
	return m
}
func plainMeta(uid, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{UID: types.UID(uid), Name: name, Namespace: "ns"}
}
func plainClusterMeta(uid, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{UID: types.UID(uid), Name: name}
}
func gvk(version, kind string) schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(version, kind)
}
func ptr[T any](v T) *T { return &v }
