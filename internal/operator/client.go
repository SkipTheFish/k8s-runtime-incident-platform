package operator

import (
	"log"
	"os"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewK8sClient() (*dynamic.DynamicClient, error) {

	config, err := rest.InClusterConfig()
	if err != nil {

		log.Println("⚠️ using local kubeconfig")

		home, _ := os.UserHomeDir()
		kubeconfig := home + "/.kube/config"

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	return dynamic.NewForConfig(config)
}