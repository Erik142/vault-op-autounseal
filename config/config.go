package config

import (
	"context"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	OnePassword OnePassword
}

type OnePassword struct {
	Host         string
	Token        string
	ItemMetadata OnePasswordItemMetadata
}

type OnePasswordItemMetadata struct {
	Name  string
	Vault string
}

const EnvOnePasswordHost = "ONEPASSWORD_HOSTNAME"
const EnvOnePasswordToken = "ONEPASSWORD_TOKEN"
const EnvOnePasswordItemName = "ONEPASSWORD_ITEM_NAME"

const DefaultOnePasswordHost = "op-connect.svc.cluster.local"
const DefaultOnePasswordToken = ""
const DefaultOnePasswordItemName = "vault"

func getEnvOrDefaultValue(envName, defaultValue string) string {
	value := os.Getenv(envName)

	if value != "" {
		return value
	}

	return defaultValue
}

func GetOnePasswordItemMetadata(kubeclient client.Client) (OnePasswordItemMetadata, error) {
	opItemName := getEnvOrDefaultValue(EnvOnePasswordItemName, DefaultOnePasswordItemName)
	opItemMetadata := OnePasswordItemMetadata{}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "onepassword.com",
		Kind:    "OnePasswordItem",
		Version: "v1",
	})

	err := kubeclient.Get(context.Background(), client.ObjectKey{
		Namespace: "vault",
		Name:      opItemName,
	}, u)

	if err != nil {
		return opItemMetadata, err
	}

	itemPath := u.Object["spec"].(map[string]interface{})["itemPath"].(string)
	itemVault := strings.Split(itemPath, "/")[1]
	itemName := strings.Split(itemPath, "/")[3]

	opItemMetadata.Vault = itemVault
	opItemMetadata.Name = itemName

	return opItemMetadata, nil
}

func GetConfig(kubeclient client.Client) (Config, error) {
	itemMetadata, err := GetOnePasswordItemMetadata(kubeclient)

	return Config{
		OnePassword: OnePassword{
			Host:         getEnvOrDefaultValue(EnvOnePasswordHost, DefaultOnePasswordHost),
			Token:        getEnvOrDefaultValue(EnvOnePasswordToken, DefaultOnePasswordToken),
			ItemMetadata: itemMetadata,
		},
	}, err
}
