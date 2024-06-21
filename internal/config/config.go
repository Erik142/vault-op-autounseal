package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	StatefulSetName      string
	StatefulSetNamespace string
	OnePassword          OnePassword
}

type OnePassword struct {
	Host         string
	Token        string
	ItemMetadata OnePasswordItemMetadata
}

type OnePasswordItemMetadata struct {
	Name      string
	Namespace string
	Vault     string
}

const EnvVaultStatefulSetName = "VAULT_STATEFULSET_NAME"
const EnvVaultStatefulSetNamespace = "VAULT_STATEFULSET_NAMESPACE"
const EnvVaultNamespace = "VAULT_NAMESPACE"
const EnvOnePasswordHost = "ONEPASSWORD_HOSTNAME"
const EnvOnePasswordToken = "ONEPASSWORD_TOKEN"
const EnvOnePasswordItemName = "ONEPASSWORD_ITEM_NAME"
const EnvOnePasswordItemNamespace = "ONEPASSWORD_ITEM_NAMESPACE"

const DefaultVaultStatefulSetName = "vault"
const DefaultVaultStatefulSetNamespace = "vault"
const DefaultOnePasswordHost = "op-connect.svc.cluster.local"
const DefaultOnePasswordToken = ""
const DefaultOnePasswordItemName = "vault"
const DefaultOnePasswordItemNamespace = "vault"

const OnePasswordItemGroup = "onepassword.com"
const OnePasswordItemKind = "OnePasswordItem"
const OnePasswordItemVersion = "v1"

const Spec = "spec"
const ItemPath = "itemPath"

func getEnvOrDefaultValue(envName, defaultValue string) string {
	value := os.Getenv(envName)

	if value != "" {
		return value
	}

	return defaultValue
}

func GetOnePasswordItemMetadata(kubeclient client.Client) (OnePasswordItemMetadata, error) {
	opItemName := getEnvOrDefaultValue(EnvOnePasswordItemName, DefaultOnePasswordItemName)
	opItemNamespace := getEnvOrDefaultValue(EnvOnePasswordItemNamespace, DefaultOnePasswordItemNamespace)
	opItemMetadata := OnePasswordItemMetadata{}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   OnePasswordItemGroup,
		Kind:    OnePasswordItemKind,
		Version: OnePasswordItemVersion,
	})

	err := kubeclient.Get(context.Background(), client.ObjectKey{
		Namespace: opItemNamespace,
		Name:      opItemName,
	}, u)

	if err != nil {
		return opItemMetadata, err
	}

	itemSpecInterface, ok := u.Object[Spec]

	if !ok {
		return opItemMetadata, fmt.Errorf("Could not retrieve 'spec' property of the OnePasswordItem named '%s'\n", opItemName)
	}

	itemSpec, ok := itemSpecInterface.(map[string]interface{})

	if !ok {
		return opItemMetadata, fmt.Errorf("Could not cast 'spec' property of the OnePasswordItem named '%s' to map[string]interface{}\n", opItemName)
	}

	itemPathInterface, ok := itemSpec[ItemPath]

	if !ok {
		return opItemMetadata, fmt.Errorf("Could not retrieve 'spec.itemPath' property of the OnePasswordItem named '%s'\n", opItemName)
	}

	itemPath, ok := itemPathInterface.(string)

	if !ok {
		return opItemMetadata, fmt.Errorf("Could not cast 'spec.itemPath' property of the OnePasswordItem named '%s' to string\n", opItemName)
	}

	itemPathParts := strings.Split(itemPath, "/")

	if len(itemPathParts) < 4 {
		return opItemMetadata, fmt.Errorf("Expected at least 4 items in itemPathParts, got %d\n", len(itemPathParts))
	}

	itemVault := itemPathParts[1]
	itemName := itemPathParts[3]

	opItemMetadata.Vault = itemVault
	opItemMetadata.Name = itemName
	opItemMetadata.Namespace = opItemNamespace

	return opItemMetadata, nil
}

func GetConfig(kubeclient client.Client) (Config, error) {
	itemMetadata, err := GetOnePasswordItemMetadata(kubeclient)

	return Config{
		StatefulSetName:      getEnvOrDefaultValue(EnvVaultStatefulSetName, DefaultVaultStatefulSetName),
		StatefulSetNamespace: getEnvOrDefaultValue(EnvVaultStatefulSetNamespace, DefaultVaultStatefulSetNamespace),
		OnePassword: OnePassword{
			Host:         getEnvOrDefaultValue(EnvOnePasswordHost, DefaultOnePasswordHost),
			Token:        getEnvOrDefaultValue(EnvOnePasswordToken, DefaultOnePasswordToken),
			ItemMetadata: itemMetadata,
		},
	}, err
}
