package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
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

	for len(keys) == 0 {
		secret, err := clientset.CoreV1().Secrets("vault").Get(context.Background(), "vault", metav1.GetOptions{})

		if err != nil {
			panic(err)
		}

		for key, value := range secret.Data {
			if strings.HasPrefix(key, "key") {
				if string(value) == "" {
					fmt.Printf("Key '%s' was empty! Trying to add secrets again in 5 seconds...\n", key)
					time.Sleep(5 * time.Second)
					keys = make([]string, 0)
					continue
				}
				keys = append(keys, string(value))
			}
		}
	}

	for true {
		statefulset, err := clientset.AppsV1().StatefulSets("vault").Get(context.Background(), "vault", metav1.GetOptions{})

		if err != nil {
			panic(err)
		}

		labelMap, err := metav1.LabelSelectorAsMap(statefulset.Spec.Selector)

		if err != nil {
			panic(err)
		}

		pods, err := clientset.CoreV1().Pods("vault").List(context.Background(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labelMap).String()})

		if err != nil {
			panic(err)
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase != "Running" {
				fmt.Printf("Pod %s is not running yet. Current status is %s\n", pod.Name, pod.Status.Phase)
				continue
			}

			sealedLabel, ok := pod.Labels["vault-sealed"]

			if !ok {
				fmt.Printf("Could not find label 'vault-sealed' for pod '%s'\n", pod.Name)
				continue
			}

			if sealedLabel == "" {
				fmt.Printf("The label 'vault-sealed' was empty for pod '%s'\n", pod.Name)
				continue
			}

			isSealed, err := strconv.ParseBool(sealedLabel)

			if err != nil {
				panic(err)
			}

			if isSealed {
				fmt.Println("Found sealed pod label for", pod.Name)
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

				if !status.Initialized {
					fmt.Println("Vault has not been initialized yet...")
					time.Sleep(5 * time.Second)
					continue
				}

				if !status.Sealed {
					fmt.Printf("Received unsealed status from Vault API for pod '%s', aborting...\n", pod.Name)
					time.Sleep(5 * time.Second)
					continue
				}

				for i < len(keys) {
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
