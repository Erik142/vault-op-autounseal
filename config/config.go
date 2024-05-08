package config

import "os"

type Config struct {
	Port        string
	Protocol    string
	OnePassword OnePassword
}

type OnePassword struct {
	Host  string
	Token string
}

const EnvVaultPort = "VAULT_PORT"
const EnvVaultProtocol = "VAULT_PROTOCOL"
const EnvOnePasswordHost = "ONEPASSWORD_HOSTNAME"
const EnvOnePasswordToken = "ONEPASSWORD_TOKEN"

const DefaultVaultPort = "8200"
const DefaultVaultProtocol = "https"
const DefaultOnePasswordHost = "op-connect.svc.cluster.local"
const DefaultOnePasswordToken = ""

func getEnvOrDefaultValue(envName, defaultValue string) string {
	value := os.Getenv(envName)

	if value != "" {
		return value
	}

	return defaultValue
}

func GetConfig() Config {
	return Config{
		Port:     getEnvOrDefaultValue(EnvVaultPort, DefaultVaultPort),
		Protocol: getEnvOrDefaultValue(EnvVaultProtocol, DefaultVaultProtocol),
		OnePassword: OnePassword{
			Host:  getEnvOrDefaultValue(EnvOnePasswordHost, DefaultOnePasswordHost),
			Token: getEnvOrDefaultValue(EnvOnePasswordToken, DefaultOnePasswordToken),
		},
	}
}
