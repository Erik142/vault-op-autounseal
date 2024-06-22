package config

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	VaultNamespace string      `yaml:"vaultNamespace"`
	OnePassword    OnePassword `yaml:"onepassword"`
}

type OnePassword struct {
	Host         string                  `yaml:"host"`
	Token        string                  `yaml:"token"`
	ItemMetadata OnePasswordItemMetadata `yaml:"secretMetadata"`
}

type OnePasswordItemMetadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	Vault     string `yaml:"vault"`
}

const EnvVaultNamespace = "VAULT_NAMESPACE"
const EnvOnePasswordHost = "ONEPASSWORD_HOSTNAME"
const EnvOnePasswordToken = "ONEPASSWORD_TOKEN"
const EnvOnePasswordItemName = "ONEPASSWORD_ITEM_NAME"
const EnvOnePasswordItemNamespace = "ONEPASSWORD_ITEM_NAMESPACE"

const DefaultVaultNamespace = "vault"
const DefaultOnePasswordHost = "op-connect.svc.cluster.local"
const DefaultOnePasswordToken = ""
const DefaultOnePasswordItemName = "vault"
const DefaultOnePasswordItemNamespace = "vault"

const OnePasswordItemGroup = "onepassword.com"
const OnePasswordItemKind = "OnePasswordItem"
const OnePasswordItemVersion = "v1"

const Spec = "spec"
const ItemPath = "itemPath"

var config *Config

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

func InitFromFile(configPath string) error {
	var configBytes []byte
	var fileinfo fs.FileInfo
	var err error

	if config == nil {
		if fileinfo, err = os.Stat(configPath); err != nil {
			return err
		}

		if fileinfo.IsDir() {
			return fmt.Errorf("The configuration file path '%s' is a directory, not a file", configPath)
		}

		if configBytes, err = os.ReadFile(configPath); err != nil {
			return err
		}

		config = new(Config)
		if err = yaml.Unmarshal(configBytes, config); err != nil {
			return err
		}
	}

	return nil
}

func Init(kubeclient client.Client) error {
	if config == nil {
		itemMetadata, err := GetOnePasswordItemMetadata(kubeclient)

		if err != nil {
			return err
		}

		config = new(Config)
		*config = Config{
			VaultNamespace: getEnvOrDefaultValue(EnvVaultNamespace, DefaultVaultNamespace),
			OnePassword: OnePassword{
				Host:         getEnvOrDefaultValue(EnvOnePasswordHost, DefaultOnePasswordHost),
				Token:        getEnvOrDefaultValue(EnvOnePasswordToken, DefaultOnePasswordToken),
				ItemMetadata: itemMetadata,
			},
		}

	}

	return nil
}

func Get() (*Config, error) {
	var err error

	if config == nil {
		err = fmt.Errorf("Configuration has not been initialized")
	}

	return config, err
}
