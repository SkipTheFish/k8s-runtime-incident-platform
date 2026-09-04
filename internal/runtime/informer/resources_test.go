package informer

import (
	"testing"
	"time"

	"observability-platform/internal/resourcegraph"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

func TestResourceTaskPathsForEveryWatchedKind(t *testing.T) {
	meta := metav1.ObjectMeta{Name: "work", Namespace: "ns", UID: types.UID("uid-1"), Labels: map[string]string{"app": "test"}, OwnerReferences: []metav1.OwnerReference{{UID: types.UID("owner-1")}}}
	objects := []struct {
		kind, apiVersion string
		object           runtime.Object
	}{
		{"Pod", "v1", &corev1.Pod{ObjectMeta: meta, Spec: corev1.PodSpec{NodeName: "node-a"}}},
		{"Node", "v1", &corev1.Node{ObjectMeta: meta}},
		{"Deployment", "apps/v1", &appsv1.Deployment{ObjectMeta: meta}},
		{"ReplicaSet", "apps/v1", &appsv1.ReplicaSet{ObjectMeta: meta}},
		{"StatefulSet", "apps/v1", &appsv1.StatefulSet{ObjectMeta: meta}},
		{"DaemonSet", "apps/v1", &appsv1.DaemonSet{ObjectMeta: meta}},
		{"Job", "batch/v1", &batchv1.Job{ObjectMeta: meta}},
	}
	now := time.Now()
	for _, item := range objects {
		for _, event := range []struct {
			name   string
			action resourcegraph.TaskAction
			wrap   bool
		}{
			{"add", resourcegraph.TaskUpsert, false},
			{"update", resourcegraph.TaskUpsert, false},
			{"delete", resourcegraph.TaskDelete, false},
			{"tombstone", resourcegraph.TaskDelete, true},
		} {
			t.Run(item.kind+"/"+event.name, func(t *testing.T) {
				var obj any = item.object
				if event.wrap {
					obj = cache.DeletedFinalStateUnknown{Key: "ns/work", Obj: item.object}
				}
				task, ok := resourceTaskFromObject("cluster", item.apiVersion, item.kind, event.action, obj, now)
				if !ok || task.Action != event.action || task.Node.Key.UID != "uid-1" || task.Node.Key.Kind != item.kind {
					t.Fatalf("unexpected task: %#v ok=%v", task, ok)
				}
				if item.kind == "Pod" && task.NodeName != "node-a" {
					t.Fatalf("node name=%q", task.NodeName)
				}
			})
		}
	}
}

func TestResourceTaskRejectsUnknownAndUIDLessObjects(t *testing.T) {
	if _, ok := resourceTaskFromObject("c", "v1", "Pod", resourcegraph.TaskUpsert, struct{}{}, time.Now()); ok {
		t.Fatal("accepted unknown object")
	}
	if _, ok := resourceTaskFromObject("c", "v1", "Pod", resourcegraph.TaskUpsert, &corev1.Pod{}, time.Now()); ok {
		t.Fatal("accepted UID-less object")
	}
}
