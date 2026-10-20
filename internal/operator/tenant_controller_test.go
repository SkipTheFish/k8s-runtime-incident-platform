package operator

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMapTenantManagedResource(t *testing.T) {
	tests := []struct {
		name      string
		labels    map[string]string
		wantCount int
	}{
		{name: "managed", labels: map[string]string{tenantSourceNamespaceLabel: "policies", tenantSourceNameLabel: "tenant-a"}, wantCount: 1},
		{name: "unmanaged", labels: map[string]string{"app": "unrelated"}, wantCount: 0},
		{name: "incomplete", labels: map[string]string{tenantSourceNameLabel: "tenant-a"}, wantCount: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := mapTenantManagedResource(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Labels: test.labels}})
			if len(requests) != test.wantCount {
				t.Fatalf("requests=%v want count=%d", requests, test.wantCount)
			}
			if test.wantCount == 1 && (requests[0].Namespace != "policies" || requests[0].Name != "tenant-a") {
				t.Fatalf("request=%v", requests[0])
			}
		})
	}
}
