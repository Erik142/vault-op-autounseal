package config

import "os"

type Config struct {
	OnePassword OnePassword
}

type OnePassword struct {
	Host  string
	Token string
}

const EnvOnePasswordHost = "ONEPASSWORD_HOSTNAME"
const EnvOnePasswordToken = "ONEPASSWORD_TOKEN"

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
		OnePassword: OnePassword{
			Host:  getEnvOrDefaultValue(EnvOnePasswordHost, DefaultOnePasswordHost),
			Token: getEnvOrDefaultValue(EnvOnePasswordToken, DefaultOnePasswordToken),
		},
	}
}
