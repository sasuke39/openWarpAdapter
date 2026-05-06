package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Provider string       `yaml:"provider"`
	BaseURL  string       `yaml:"base_url"`
	APIKey   string       `yaml:"api_key"`
	Model    string       `yaml:"model"`
	Server   ServerConfig `yaml:"server"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 18888,
		},
	}
}

func ApplyDefaults(cfg *Config) *Config {
	if cfg == nil {
		cfg = Default()
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "127.0.0.1"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 18888
	}
	return cfg
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return ApplyDefaults(&cfg), nil
}

func LoadOrDefault(path string) (*Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return Default(), err
	}
	return ApplyDefaults(cfg), nil
}

func Dump(cfg *Config) ([]byte, error) {
	cfg = ApplyDefaults(cfg)
	return yaml.Marshal(cfg)
}

func MissingRequiredFields(cfg *Config) []string {
	cfg = ApplyDefaults(cfg)
	var missing []string
	if strings.TrimSpace(cfg.Provider) == "" {
		missing = append(missing, "provider")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		missing = append(missing, "base_url")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		missing = append(missing, "api_key")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		missing = append(missing, "model")
	}
	if strings.TrimSpace(cfg.Server.Host) == "" {
		missing = append(missing, "server.host")
	}
	if cfg.Server.Port == 0 {
		missing = append(missing, "server.port")
	}
	return missing
}

func IsConfigured(cfg *Config) bool {
	return len(MissingRequiredFields(cfg)) == 0
}
