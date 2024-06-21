package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Erik142/vault-op-autounseal/internal/config"
	"github.com/Erik142/vault-op-autounseal/internal/vault"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var Config *config.Config

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

	client, err := client.New(restConfig, client.Options{})

	if err != nil {
		panic(fmt.Errorf("Could not create Kubernetes client: %v\n", err))
	}

	clientset := kubernetes.NewForConfigOrDie(restConfig)

	err = config.Init(client)

	if err != nil {
		panic(fmt.Errorf("Could not create application configuration: %v\n", err))
	}

	Config, err = config.Get()

	if err != nil {
		panic(err)
	}

	for true {
		_, err := clientset.AppsV1().StatefulSets(Config.StatefulSetNamespace).Get(context.Background(), Config.StatefulSetName, metav1.GetOptions{})

		if err != nil {
			fmt.Printf("Waiting for Vault Statefulset '%s' to be created...\n", Config.StatefulSetName)
			time.Sleep(5 * time.Second)
			continue
		}

		fmt.Printf("Found Vault Statefulset '%s'!\n", Config.StatefulSetName)
		break
	}

	for true {
		vault, err := vault.New(clientset)

		if err != nil {
			fmt.Println(err)
			time.Sleep(5 * time.Second)
			continue
		}

		if err = vault.Init(); err != nil {
			fmt.Println(err)
			time.Sleep(5 * time.Second)
			continue
		}

		if err = vault.Unseal(); err != nil {
			fmt.Println(err)
		}

		time.Sleep(5 * time.Second)
	}
}
