package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		WSURL      string `yaml:"ws_url"`
		JarBaseURL string `yaml:"jar_base_url"`
	} `yaml:"server"`
	Agent struct {
		ID        string `yaml:"id"`
		Workspace string `yaml:"workspace"`
		LogDir    string `yaml:"log_dir"`
	} `yaml:"agent"`
	Devices  []DeviceConfig  `yaml:"devices"`
	Services []ServiceConfig `yaml:"services"`
}

type DeviceConfig struct {
	ID      string `yaml:"id"`
	Host    string `yaml:"host"`
	SSHUser string `yaml:"ssh_user"`
	SSHPort int    `yaml:"ssh_port"`
	TempDir string `yaml:"temp_dir"`
}

type ServiceConfig struct {
	DeviceID      string `yaml:"device_id"`
	ServiceName   string `yaml:"service_name"`
	DeployDir     string `yaml:"deploy_dir"`
	TargetJarName string `yaml:"target_jar_name"`
	StartScript   string `yaml:"start_script"`
	StopScript    string `yaml:"stop_script"`
	ProcessName   string `yaml:"process_name"`
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

	if cfg.Server.WSURL == "" {
		return Config{}, fmt.Errorf("server.ws_url is required")
	}
	if cfg.Agent.ID == "" {
		return Config{}, fmt.Errorf("agent.id is required")
	}
	if cfg.Agent.Workspace == "" {
		return Config{}, fmt.Errorf("agent.workspace is required")
	}
	if cfg.Agent.LogDir == "" {
		return Config{}, fmt.Errorf("agent.log_dir is required")
	}
	for i := range cfg.Devices {
		if cfg.Devices[i].SSHPort == 0 {
			cfg.Devices[i].SSHPort = 22
		}
	}
	return cfg, nil
}
