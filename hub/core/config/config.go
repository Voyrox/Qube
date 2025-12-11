package config

import (
	"os"
	"strconv"
)

type Config struct {
	Addr  string
	Debug bool

	ScyllaHosts    []string
	ScyllaKeyspace string
	ScyllaUsername string
	ScyllaPassword string

	JWTSecret string

	StoragePath   string
	MaxUploadSize int64
}

func Load() *Config {
	return &Config{
		Addr:  getEnv("ADDR", ":2112"),
		Debug: getEnv("DEBUG", "false") == "true",

		ScyllaHosts:    getEnvArray("SCYLLA_HOSTS", []string{"127.0.0.1"}),
		ScyllaKeyspace: getEnv("SCYLLA_KEYSPACE", "qube_hub"),
		ScyllaUsername: getEnv("SCYLLA_USERNAME", ""),
		ScyllaPassword: getEnv("SCYLLA_PASSWORD", ""),

		JWTSecret: getEnv("JWT_SECRET", "your-secret-key-change-this"),

		StoragePath:   getEnv("STORAGE_PATH", "./storage/images"),
		MaxUploadSize: getEnvInt64("MAX_UPLOAD_SIZE", 1073741824), // 1GB default
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvArray(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		hosts := []string{}
		for _, host := range splitString(value, ",") {
			hosts = append(hosts, host)
		}
		return hosts
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func splitString(s, sep string) []string {
	result := []string{}
	current := ""
	for _, char := range s {
		if string(char) == sep {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
