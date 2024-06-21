package main

import (
	"flag"
	"path/filepath"

	"github.com/Erik142/vault-op-autounseal/internal/controller"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	restConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)

	if err != nil {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}

	c, err := controller.New(restConfig)

	if err := c.Reconcile(); err != nil {
		panic(err)
	}
}
