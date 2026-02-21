package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	JWT        JWTConfig        `yaml:"jwt"`
	CORS       CORSConfig       `yaml:"cors"`
	Session    SessionConfig    `yaml:"session"`
	Encryption EncryptionConfig `yaml:"encryption"`
	Logging    LoggingConfig    `yaml:"logging"`
	Stdio      StdioConfig      `yaml:"stdio"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Telemetry  TelemetryConfig  `yaml:"telemetry"`
}

type TelemetryConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Endpoint    string `yaml:"endpoint"`
	ServiceName string `yaml:"service_name"`
	Insecure    bool   `yaml:"insecure"`
}

type StdioConfig struct {
	IdleTTL      time.Duration `yaml:"idle_ttl"`
	MaxLifetime  time.Duration `yaml:"max_lifetime"`
	GCInterval   time.Duration `yaml:"gc_interval"`
	MaxProcesses int           `yaml:"max_processes"`
}

type KubernetesConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Namespace    string        `yaml:"namespace"`
	Kubeconfig   string        `yaml:"kubeconfig"`
	IdleTTL      time.Duration `yaml:"idle_ttl"`
	MaxLifetime  time.Duration `yaml:"max_lifetime"`
	GCInterval   time.Duration `yaml:"gc_interval"`
	MaxInstances int           `yaml:"max_instances"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"sslmode"`
}

type JWTConfig struct {
	Secret string `yaml:"secret"`
}

type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedHeaders []string `yaml:"allowed_headers"`
	ExposeHeaders  []string `yaml:"expose_headers"`
}

type SessionConfig struct {
	Timeout         time.Duration `yaml:"timeout"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

type EncryptionConfig struct {
	Key string `yaml:"key"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Load reads the config file and expands environment variables
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Expand environment variables in format ${VAR_NAME}
	expanded := expandEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	setDefaults(&cfg)

	return &cfg, nil
}

// expandEnvVars replaces ${VAR_NAME} with the value of the environment variable
func expandEnvVars(s string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		varName := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match
	})
}

func setDefaults(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 3000
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Session.Timeout == 0 {
		cfg.Session.Timeout = 30 * time.Minute
	}
	if cfg.Session.CleanupInterval == 0 {
		cfg.Session.CleanupInterval = 5 * time.Minute
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
	if cfg.Stdio.IdleTTL == 0 {
		cfg.Stdio.IdleTTL = 30 * time.Minute
	}
	if cfg.Stdio.MaxLifetime == 0 {
		cfg.Stdio.MaxLifetime = 24 * time.Hour
	}
	if cfg.Stdio.GCInterval == 0 {
		cfg.Stdio.GCInterval = 1 * time.Minute
	}
	if cfg.Stdio.MaxProcesses == 0 {
		cfg.Stdio.MaxProcesses = 100
	}
	if cfg.Kubernetes.Namespace == "" {
		cfg.Kubernetes.Namespace = "reflow"
	}
	if cfg.Kubernetes.IdleTTL == 0 {
		cfg.Kubernetes.IdleTTL = 30 * time.Minute
	}
	if cfg.Kubernetes.MaxLifetime == 0 {
		cfg.Kubernetes.MaxLifetime = 24 * time.Hour
	}
	if cfg.Kubernetes.GCInterval == 0 {
		cfg.Kubernetes.GCInterval = 1 * time.Minute
	}
	if cfg.Kubernetes.MaxInstances == 0 {
		cfg.Kubernetes.MaxInstances = 100
	}
	if cfg.Telemetry.ServiceName == "" {
		cfg.Telemetry.ServiceName = "reflow-gateway"
	}
	if cfg.Telemetry.Endpoint == "" {
		cfg.Telemetry.Endpoint = "localhost:4317"
	}
}

// GetDSN returns the PostgreSQL connection string
func (d *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Database, d.SSLMode)
}
