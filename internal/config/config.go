package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AppKey      string `yaml:"app_key"`
	AppSecret   string `yaml:"app_secret"`
	AccessToken string `yaml:"access_token"`
	ServerHost  string `yaml:"host"`
	ServerPort  int    `yaml:"port"`
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

	// Use environment variables as fallback
	if cfg.AppKey == "" {
		cfg.AppKey = os.Getenv("LONGPORT_APP_KEY")
	}
	if cfg.AppSecret == "" {
		cfg.AppSecret = os.Getenv("LONGPORT_APP_SECRET")
	}
	if cfg.AccessToken == "" {
		cfg.AccessToken = os.Getenv("LONGPORT_ACCESS_TOKEN")
	}

	// Set defaults
	if cfg.ServerHost == "" {
		cfg.ServerHost = "0.0.0.0"
	}
	if cfg.ServerPort == 0 {
		cfg.ServerPort = 8080
	}

	return &cfg, nil
}
