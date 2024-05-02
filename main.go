package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/1Password/connect-sdk-go/connect"
	"github.com/1Password/connect-sdk-go/onepassword"
	"github.com/hashicorp/vault/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const VAULT_ADDRESS_TEMPLATE = "http://%s:8200"

type Vault struct {
	Keys        []string
	IpAddresses []string
}

var OP_CONNECT_HOST string = "http://op-connect.k8s.gbg.wahlberger.lan"
var OP_CONNECT_TOKEN string = ""

func NewVault(clientset *kubernetes.Clientset) (*Vault, error) {
	keys, _ := GetVaultKeysFromSecret(clientset)
	ipaddrs, err := GetVaultPodIpAddresses(clientset)

	if err != nil {
		return nil, err
	}

	return &Vault{Keys: keys, IpAddresses: ipaddrs}, nil
}

func NewVaultApiClient(ipaddr string) (*api.Client, error) {
	vaultconfig := api.DefaultConfig()
	vaultconfig.Address = fmt.Sprintf(VAULT_ADDRESS_TEMPLATE, ipaddr)
	return api.NewClient(vaultconfig)
}

func GetVaultKeysFromSecret(clientset *kubernetes.Clientset) ([]string, error) {
	keys := make([]string, 0)
	secret, err := clientset.CoreV1().Secrets("vault").Get(context.Background(), "vault", metav1.GetOptions{})

	if err != nil {
		return keys, err
	}

	for key, value := range secret.Data {
		if strings.HasPrefix(key, "key") {
			if string(value) == "" {
				return keys, fmt.Errorf("Key '%s' value is empty.\n", key)
			}
			keys = append(keys, string(value))
		}
	}

	return keys, nil
}

func GetOnepasswordTokenFromSecret(clientset *kubernetes.Clientset) (string, error) {
	secret, err := clientset.CoreV1().Secrets("vault").Get(context.Background(), "onepassword-token", metav1.GetOptions{})

	if err != nil {
		return "", err
	}

	return string(secret.Data["onepassword-token"]), nil
}

func GetVaultPodIpAddresses(clientset *kubernetes.Clientset) ([]string, error) {
	ipaddrs := make([]string, 0)

	statefulset, err := clientset.AppsV1().StatefulSets("vault").Get(context.Background(), "vault", metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	labelMap, err := metav1.LabelSelectorAsMap(statefulset.Spec.Selector)

	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods("vault").List(context.Background(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labelMap).String()})

	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != "Running" {
			return nil, fmt.Errorf("Pod %s is not running yet. Current status is %s\n", pod.Name, pod.Status.Phase)
		}

		ipaddrs = append(ipaddrs, pod.Status.PodIP)
	}

	return ipaddrs, nil
}

func PushVaultKeys(initResponse *api.InitResponse) error {
	client := connect.NewClient(OP_CONNECT_HOST, OP_CONNECT_TOKEN)

	secret, err := client.GetItem("Vault", "DevOps")

	if err != nil {
		for i, key := range initResponse.Keys {
			fmt.Printf("key-%d: %s\n", (i + 1), key)
		}

		for i, key := range initResponse.RecoveryKeys {
			fmt.Printf("recoverykey-%d: %s\n", (i + 1), key)
		}

		fmt.Printf("root-token: %s\n", initResponse.RootToken)

		return fmt.Errorf("Could not update 1password Vault keys: %s\n", err)
	}

	updatedFields := make([]*onepassword.ItemField, 0)

	for _, field := range secret.Fields {
		label := field.Label

		if strings.HasPrefix(label, "key-") {
			index, err := strconv.Atoi(strings.ReplaceAll(label, "key-", ""))

			if err != nil {
				return fmt.Errorf("Could not parse 1Password secret key index: %s\n", err)
			}

			index = index - 1

			field.Value = initResponse.Keys[index]
			updatedFields = append(updatedFields, field)
		}
	}

	secret.Fields = updatedFields
	client.UpdateItem(secret, "")

	return nil
}

func (self *Vault) Init() error {
	isInitialized := true

	for _, ipaddr := range self.IpAddresses {
		client, err := NewVaultApiClient(ipaddr)

		if err != nil {
			return fmt.Errorf("Could not create Vault API client for Pod with IP address: %s\n", ipaddr)
		}

		initStatus, err := client.Sys().InitStatus()

		if err != nil {
			return fmt.Errorf("Could not retrieve init status for Pod with IP address: %s\n", ipaddr)
		}

		isInitialized = isInitialized && initStatus
	}

	if !isInitialized {
		client, err := NewVaultApiClient(self.IpAddresses[0])

		if err != nil {
			return fmt.Errorf("Could not create Vault API client for Pod with IP address: %s\n", self.IpAddresses[0])
		}

		initResult, err := client.Sys().InitWithContext(context.Background(), &api.InitRequest{})

		if err != nil {
			return fmt.Errorf("Could not initialize Vault: %s\n", err)
		}

		self.Keys = initResult.Keys

		return PushVaultKeys(initResult)
	}

	return nil
}

func (self *Vault) Unseal() error {
	for _, ipaddr := range self.IpAddresses {
		client, err := NewVaultApiClient(ipaddr)

		if err != nil {
			return fmt.Errorf("Could not create Vault API client for Pod with IP address: %s\n", ipaddr)
		}

		sealStatus, err := client.Sys().SealStatus()

		if err != nil {
			return fmt.Errorf("Could not retrieve Vault seal status: %s\n", err)
		}

		if !sealStatus.Sealed {
			continue
		}

		fmt.Println("Found unsealed Vault Pod with IP address: %s\n", ipaddr)

		for _, key := range self.Keys {
			sealResponse, err := client.Sys().Unseal(key)

			if err != nil {
				return fmt.Errorf("Could not unseal Vault Pod with IP address: %s - %s\n", ipaddr, err)
			}

			if !sealResponse.Initialized {
				return fmt.Errorf("Could not unseal Vault Pod with IP address: %s - Vault Pod is not initialized\n", ipaddr)
			}

			if sealResponse.Sealed {
				return fmt.Errorf("Could not unseal Vault Pod with IP address: %s\n", ipaddr)
			}
		}
	}

	return nil
}

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

	opToken, err := GetOnepasswordTokenFromSecret(clientset)

	if err != nil {
		panic(err)
	}

	OP_CONNECT_TOKEN = opToken

	for true {
		vault, err := NewVault(clientset)

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
