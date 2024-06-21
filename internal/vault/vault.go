package vault

import (
	"context"
	"fmt"
	"strings"

	"github.com/Erik142/vault-op-autounseal/internal/config"
	"github.com/Erik142/vault-op-autounseal/internal/onepassword"
	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type Vault struct {
	Config       *config.Config
	Keys         []string
	ApiAddresses []string
}

func New(clientset *kubernetes.Clientset) (*Vault, error) {
	keys, _ := getKeysFromSecret(clientset)
	apiaddrs, err := getPodApiAddresses(clientset)

	if err != nil {
		return nil, err
	}

	config, err := config.Get()

	if err != nil {
		return nil, err
	}

	return &Vault{Config: config, Keys: keys, ApiAddresses: apiaddrs}, nil
}

func newApiClient(apiaddr string) (*api.Client, error) {
	vaultconfig := api.DefaultConfig()
	vaultconfig.Address = apiaddr
	vaultconfig.ConfigureTLS(&api.TLSConfig{Insecure: true})
	return api.NewClient(vaultconfig)
}

func getKeysFromSecret(clientset *kubernetes.Clientset) ([]string, error) {
	keys := make([]string, 0)
	c, err := config.Get()

	if err != nil {
		return nil, err
	}

	secret, err := clientset.CoreV1().Secrets(c.OnePassword.ItemMetadata.Namespace).Get(context.Background(), c.OnePassword.ItemMetadata.Name, metav1.GetOptions{})

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

func getOnePasswordKeyMap(initResponse *api.InitResponse) map[string]string {
	vaultKeys := make(map[string]string, 0)

	for i, key := range initResponse.KeysB64 {
		mapKey := fmt.Sprintf("key-%d", (i + 1))
		vaultKeys[mapKey] = key
	}

	for i, key := range initResponse.RecoveryKeysB64 {
		mapKey := fmt.Sprintf("recoverykey-%d", (i + 1))
		vaultKeys[mapKey] = key
	}

	vaultKeys["root-token"] = initResponse.RootToken

	return vaultKeys
}

func getPodApiAddresses(clientset *kubernetes.Clientset) ([]string, error) {
	apiaddrs := make([]string, 0)
	c, err := config.Get()

	if err != nil {
		return nil, err
	}

	statefulset, err := clientset.AppsV1().StatefulSets(c.StatefulSetNamespace).Get(context.Background(), c.StatefulSetName, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	labelMap, err := metav1.LabelSelectorAsMap(statefulset.Spec.Selector)

	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods(c.StatefulSetNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labelMap).String()})

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
						apiaddrs = append(apiaddrs, apiaddr)

						log.Debugf("Found Vault API address: %v", apiaddr)
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

func (self *Vault) Init() error {
	isInitialized := true

	for _, apiaddr := range self.ApiAddresses {
		client, err := newApiClient(apiaddr)

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
		client, err := newApiClient(self.ApiAddresses[0])

		if err != nil {
			return fmt.Errorf("Could not create Vault API client for Pod with API address: %v - %v\n", self.ApiAddresses[0], err)
		}

		initResult, err := client.Sys().InitWithContext(context.Background(), &api.InitRequest{SecretShares: 5, SecretThreshold: 3})

		if err != nil {
			return fmt.Errorf("Could not initialize Vault: %v\n", err)
		}

		self.Keys = initResult.KeysB64

		opKeys := getOnePasswordKeyMap(initResult)

		return onepassword.UpdateSecret(opKeys)
	}

	return nil
}

func (self *Vault) Unseal() error {
	for _, apiaddr := range self.ApiAddresses {
		client, err := newApiClient(apiaddr)

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

		log.Infof("Found sealed Vault Pod with API address: %v", apiaddr)

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
