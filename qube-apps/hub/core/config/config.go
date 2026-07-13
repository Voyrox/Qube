package config

import (
	"os"
	"strconv"
)

type Config struct {
	Addr  string
	Debug bool

	ScyllaHosts       []string
	ScyllaKeyspace    string
	ScyllaLocalDC     string
	ScyllaUsername    string
	ScyllaPassword    string
	ScyllaTLS         bool
	ScyllaTLSCA       string
	ScyllaTLSCert     string
	ScyllaTLSKey      string
	ScyllaTLSInsecure bool

	JWTSecret string

	StoragePath   string
	MaxUploadSize int64

	DisableAtMinReports int

	AdminEmail string
}

func Load() *Config {
	return &Config{
		Addr:  getEnv("ADDR", ":32003"),
		Debug: getEnv("DEBUG", "false") == "true",

		ScyllaHosts:       getEnvArray("SCYLLA_HOSTS", []string{"10.50.0.2", "10.50.0.3"}),
		ScyllaKeyspace:    getEnv("SCYLLA_KEYSPACE", "qube_hub"),
		ScyllaLocalDC:     getEnv("SCYLLA_LOCAL_DC", "Australia"),
		ScyllaUsername:    getEnv("SCYLLA_USERNAME", "ewen"),
		ScyllaPassword:    getEnv("SCYLLA_PASSWORD", "Tensor$35a"),
		ScyllaTLS:         getEnv("SCYLLA_TLS", "false") == "true",
		ScyllaTLSCA:       getEnv("SCYLLA_TLS_CA_PATH", ""),
		ScyllaTLSCert:     getEnv("SCYLLA_TLS_CERT_PATH", ""),
		ScyllaTLSKey:      getEnv("SCYLLA_TLS_KEY_PATH", ""),
		ScyllaTLSInsecure: getEnv("SCYLLA_TLS_INSECURE", "false") == "true",

		JWTSecret: getEnv("JWT_SECRET", "cd609f60f2fd459cc82ca31f789da20a1a2fafa6807896b83f625a6279fc3102e2e0fcde"),

		StoragePath:   getEnv("STORAGE_PATH", "./storage/images"),
		MaxUploadSize: getEnvInt64("MAX_UPLOAD_SIZE", 5368709120), // 5GB default

		DisableAtMinReports: getEnvInt("DISABLE_AT_MIN_REPORTS", 5), // Default 5 reports

		AdminEmail: getEnv("ADMIN_EMAIL", "ewen@macculloch.net"),
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

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
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
