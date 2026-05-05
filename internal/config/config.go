package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Provider string     `yaml:"provider"`
	BaseURL  string     `yaml:"base_url"`
	APIKey   string     `yaml:"api_key"`
	Model    string     `yaml:"model"`
	Server   ServerConfig `yaml:"server"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
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
	return &cfg, nil
}
