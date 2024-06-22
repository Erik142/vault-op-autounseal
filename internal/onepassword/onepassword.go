package onepassword

import (
	"context"
	"fmt"
	"strings"

	"github.com/1Password/connect-sdk-go/connect"
	"github.com/1Password/connect-sdk-go/onepassword"
	"github.com/Erik142/vault-op-autounseal/internal/config"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetKeysFromSecret(clientset *kubernetes.Clientset) ([]string, error) {
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

func UpdateSecret(keys map[string]string) error {
	Config, err := config.Get()

	if err != nil {
		return err
	}

	client := connect.NewClient(Config.OnePassword.Host, Config.OnePassword.Token)

	secret, err := client.GetItem(Config.OnePassword.ItemMetadata.Name, Config.OnePassword.ItemMetadata.Vault)

	if err != nil {
		for name, key := range keys {
			log.Infof("%v: %v", name, key)
		}

		return fmt.Errorf("Could not update 1password Vault keys: %v\n", err)
	}

	updatedFields := make([]*onepassword.ItemField, 0)

	for _, field := range secret.Fields {
		label := field.Label

		if strings.Contains(label, "key-") || label == "root-token" {
			key, ok := keys[label]

			if !ok {
				continue
			}

			field.Value = key
		}

		updatedFields = append(updatedFields, field)
	}

	secret.Fields = updatedFields
	client.UpdateItem(secret, "")

	return nil
}
