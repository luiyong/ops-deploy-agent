package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr    string `yaml:"listen_addr"`
	PublicBaseURL string `yaml:"public_base_url"`
	JarDir        string `yaml:"jar_dir"`
	RecordFile    string `yaml:"record_file"`
	WSPath        string `yaml:"ws_path"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if cfg.ListenAddr == "" {
		return Config{}, fmt.Errorf("listen_addr is required")
	}
	if cfg.JarDir == "" {
		return Config{}, fmt.Errorf("jar_dir is required")
	}
	if cfg.RecordFile == "" {
		return Config{}, fmt.Errorf("record_file is required")
	}
	if cfg.WSPath == "" {
		return Config{}, fmt.Errorf("ws_path is required")
	}

	return cfg, nil
}
