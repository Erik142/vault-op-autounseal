package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hashicorp/vault/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const VAULT_ADDRESS_TEMPLATE = "http://%s:8200"

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)

	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}

	clientset := kubernetes.NewForConfigOrDie(config)
	vaultconfig := api.DefaultConfig()

	keys := make([]string, 0)

  secret, err := clientset.CoreV1().Secrets("vault").Get(context.TODO(), "vault", metav1.GetOptions{})

  if err != nil {
    panic(err)
  }

  for _, value := range secret.Data {
    keys = append(keys, string(value))
  }

	for true {
		statefulset, err := clientset.AppsV1().StatefulSets("vault").Get(context.TODO(), "vault", metav1.GetOptions{})

		if err != nil {
			panic(err)
		}

		labelMap, err := metav1.LabelSelectorAsMap(statefulset.Spec.Selector)

		if err != nil {
			panic(err)
		}

		pods, err := clientset.CoreV1().Pods("vault").List(context.TODO(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labelMap).String()})

		if err != nil {
			panic(err)
		}

		for _, pod := range pods.Items {
			label, ok := pod.Labels["vault-sealed"]

			if !ok {
				fmt.Printf("Could not find label 'vault-sealed' for pod '%s'\n", pod.Name)
				continue
			}

			if label == "" {
				fmt.Printf("The label 'vault-sealed' was empty for pod '%s'\n", pod.Name)
				continue
			}

			isSealed, err := strconv.ParseBool(label)

			if err != nil {
				panic(err)
			}

			if isSealed {
				fmt.Println("Found unsealed pod", pod.Name)
				vaultconfig.Address = fmt.Sprintf(VAULT_ADDRESS_TEMPLATE, pod.Status.PodIP)
				vault, err := api.NewClient(vaultconfig)

				if err != nil {
					fmt.Printf("ERROR: %v\n", err)
					continue
				}

				vaultSys := vault.Sys()
				status, err := vaultSys.SealStatus()

				if err != nil {
					fmt.Printf("ERROR: %v\n", err)
					continue
				}

				i := 0

				for status.Sealed && i < len(keys) {
					status, err = vaultSys.Unseal(keys[i])
					if err != nil {
						fmt.Printf("ERROR: %v\n", err)
					}
					i++
				}

				if !status.Sealed {
					fmt.Printf("Pod %s has successfully been unsealed!\n", pod.Name)
				}
			}
		}

		time.Sleep(5 * time.Second)
	}
}
