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
	"github.com/Erik142/vault-op-autounseal/internal/config"
	"github.com/hashicorp/vault/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Vault struct {
	Keys         []string
	ApiAddresses []string
}

var Config config.Config

func NewVault(clientset *kubernetes.Clientset) (*Vault, error) {
	keys, _ := GetVaultKeysFromSecret(clientset)
	apiaddrs, err := GetVaultPodApiAddresses(clientset)

	if err != nil {
		return nil, err
	}

	return &Vault{Keys: keys, ApiAddresses: apiaddrs}, nil
}

func NewVaultApiClient(apiaddr string) (*api.Client, error) {
	vaultconfig := api.DefaultConfig()
	vaultconfig.Address = apiaddr
	vaultconfig.ConfigureTLS(&api.TLSConfig{Insecure: true})
	return api.NewClient(vaultconfig)
}

func GetVaultKeysFromSecret(clientset *kubernetes.Clientset) ([]string, error) {
	keys := make([]string, 0)
	secret, err := clientset.CoreV1().Secrets(Config.OnePassword.ItemMetadata.Namespace).Get(context.Background(), Config.OnePassword.ItemMetadata.Name, metav1.GetOptions{})

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

func GetVaultPodApiAddresses(clientset *kubernetes.Clientset) ([]string, error) {
	apiaddrs := make([]string, 0)

	statefulset, err := clientset.AppsV1().StatefulSets(Config.StatefulSetNamespace).Get(context.Background(), Config.StatefulSetName, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	labelMap, err := metav1.LabelSelectorAsMap(statefulset.Spec.Selector)

	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods(Config.StatefulSetNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labelMap).String()})

	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != "Running" {
			return nil, fmt.Errorf("Pod %s is not running yet. Current status is %s\n", pod.Name, pod.Status.Phase)
		}

		for _, container := range pod.Spec.Containers {
			if container.Name == "vault" {
				for _, env := range container.Env {
					if env.Name == "VAULT_API_ADDR" {
						apiaddr := env.Value
						apiaddr = strings.ReplaceAll(apiaddr, "$(POD_IP)", pod.Status.PodIP)
						fmt.Printf("Found Vault API address: %v\n", apiaddr)
						apiaddrs = append(apiaddrs, apiaddr)
						break
					}
				}
			}
		}
	}

	if len(apiaddrs) == 0 {
		return nil, fmt.Errorf("Could not find Vault API addresses")
	}

	return apiaddrs, nil
}

func PushVaultKeys(initResponse *api.InitResponse) error {
	client := connect.NewClient(Config.OnePassword.Host, Config.OnePassword.Token)

	secret, err := client.GetItem(Config.OnePassword.ItemMetadata.Name, Config.OnePassword.ItemMetadata.Vault)

	if err != nil {
		for i, key := range initResponse.KeysB64 {
			fmt.Printf("key-%d: %v\n", (i + 1), key)
		}

		for i, key := range initResponse.RecoveryKeysB64 {
			fmt.Printf("recoverykey-%d: %v\n", (i + 1), key)
		}

		fmt.Printf("root-token: %v\n", initResponse.RootToken)

		return fmt.Errorf("Could not update 1password Vault keys: %v\n", err)
	}

	updatedFields := make([]*onepassword.ItemField, 0)

	for _, field := range secret.Fields {
		label := field.Label

		if strings.HasPrefix(label, "key-") {
			index, err := strconv.Atoi(strings.ReplaceAll(label, "key-", ""))

			if err != nil {
				return fmt.Errorf("Could not parse 1Password secret key index: %v\n", err)
			}

			index = index - 1

			field.Value = initResponse.KeysB64[index]
		}

		if label == "root-token" {
			field.Value = initResponse.RootToken
		}

		updatedFields = append(updatedFields, field)
	}

	secret.Fields = updatedFields
	client.UpdateItem(secret, "")

	return nil
}

func (self *Vault) Init() error {
	isInitialized := true

	for _, apiaddr := range self.ApiAddresses {
		client, err := NewVaultApiClient(apiaddr)

		if err != nil {
			return fmt.Errorf("Could not create Vault API client for Pod with API address: %v - %v\n", apiaddr, err)
		}

		initStatus, err := client.Sys().InitStatus()

		if err != nil {
			return fmt.Errorf("Could not retrieve init status for Pod with API address: %v - %v\n", apiaddr, err)
		}

		isInitialized = isInitialized && initStatus
	}

	if !isInitialized {
		client, err := NewVaultApiClient(self.ApiAddresses[0])

		if err != nil {
			return fmt.Errorf("Could not create Vault API client for Pod with API address: %v - %v\n", self.ApiAddresses[0], err)
		}

		initResult, err := client.Sys().InitWithContext(context.Background(), &api.InitRequest{SecretShares: 5, SecretThreshold: 3})

		if err != nil {
			return fmt.Errorf("Could not initialize Vault: %v\n", err)
		}

		self.Keys = initResult.KeysB64

		return PushVaultKeys(initResult)
	}

	return nil
}

func (self *Vault) Unseal() error {
	for _, apiaddr := range self.ApiAddresses {
		client, err := NewVaultApiClient(apiaddr)

		if err != nil {
			return fmt.Errorf("Could not create Vault API client for Pod with API address: %v\n", apiaddr)
		}

		sealStatus, err := client.Sys().SealStatus()

		if err != nil {
			return fmt.Errorf("Could not retrieve Vault seal status: %v\n", err)
		}

		if !sealStatus.Sealed {
			continue
		}

		fmt.Printf("Found sealed Vault Pod with API address: %v\n", apiaddr)

		for _, key := range self.Keys {
			sealResponse, err := client.Sys().Unseal(key)

			if err != nil {
				return fmt.Errorf("Could not unseal Vault Pod with API address: %v - %v\n", apiaddr, err)
			}

			if !sealResponse.Sealed {
				break
			}
		}

		sealStatus, err = client.Sys().SealStatus()

		if err != nil {
			return fmt.Errorf("Could not retrieve Vault seal status: %v\n", err)
		}

		if sealStatus.Sealed {
			return fmt.Errorf("Could not unseal Vault Pod with API address: %v\n", apiaddr)
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

	Config, err = config.GetConfig(client)

	if err != nil {
		panic(fmt.Errorf("Could not create application configuration: %v\n", err))
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
