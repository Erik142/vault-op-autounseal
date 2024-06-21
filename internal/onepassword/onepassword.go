package onepassword

import (
	"fmt"
	"github.com/1Password/connect-sdk-go/connect"
	"github.com/1Password/connect-sdk-go/onepassword"
	"github.com/Erik142/vault-op-autounseal/internal/config"
	log "github.com/sirupsen/logrus"
	"strings"
)

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
