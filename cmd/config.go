package cmd

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type StorageConfig struct {
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Region    string `yaml:"region"`
	Prefix    string `yaml:"prefix"`
}

type FileConfig struct {
	Source      StorageConfig `yaml:"source"`
	Destination StorageConfig `yaml:"destination"`
	Concurrency int           `yaml:"concurrency"`
	PartSizeMiB int64         `yaml:"part_size_mib"`
}

func loadConfig(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}
