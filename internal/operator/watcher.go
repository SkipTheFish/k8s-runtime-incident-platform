package operator

import (
	"context"
	"log"
	"time"
	"k8s.io/client-go/dynamic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var gvr = schema.GroupVersionResource{
	Group:    "obs.io",
	Version:  "v1",
	Resource: "alertpolicies",
}

func Watch(client dynamic.Interface, controller *Controller) {

	for {
		list, err := client.Resource(gvr).List(
			context.Background(),
			metav1.ListOptions{},
		)

		if err != nil {
			log.Println(err)
			time.Sleep(10 * time.Second)
			continue
		}

		log.Println("CR count:", len(list.Items))

		for _, item := range list.Items {

			log.Println(
				"found CR:",
				item.GetNamespace(),
				item.GetName(),
			)

			spec, ok := item.Object["spec"].(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := spec["name"].(string)
			query, _ := spec["query"].(string)
			threshold, _ := spec["threshold"].(float64)
			operator, _ := spec["operator"].(string)

			controller.Sync(
				name,
				query,
				threshold,
				operator,
			)
		}

		time.Sleep(10 * time.Second)
	}
}