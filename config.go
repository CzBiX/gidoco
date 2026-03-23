package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	Debug     bool
	RepoRoot  string
	Port      int
	DryRun    bool
	NoStartUp bool

	EncryptionEnabled bool

	WebhookId string

	GitUrl        string
	GitToken      string
	GitSshKey     string
	GitSshKeyFile string
}

type RepoConfig struct {
	IncludedProjects []string
	ExcludedProjects []string
}

func loadRepoConfig(repoRoot string) (*RepoConfig, error) {
	v := viper.New()
	v.SetConfigName(".gidoco")
	v.SetConfigType("yml")
	v.AddConfigPath(repoRoot)

	if err := v.ReadInConfig(); err != nil {
		if errors.Is(err, viper.ConfigFileNotFoundError{}) {
			slog.Debug("repo config file not found")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read repo config: %w", err)
	}

	var rc RepoConfig
	if err := v.Unmarshal(&rc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal repo config: %w", err)
	}

	if len(rc.IncludedProjects) > 0 && len(rc.ExcludedProjects) > 0 {
		return nil, errors.New("included projects and excluded projects are mutually exclusive")
	}

	slog.Debug("repo config loaded", "config", rc)
	return &rc, nil
}

func checkConfig(config Config) error {
	if config.RepoRoot == "" {
		return errors.New("repo root is not set")
	}

	if !filepath.IsAbs(config.RepoRoot) {
		return errors.New("repo root is not an absolute path")
	}

	if config.GitSshKeyFile != "" {
		if config.GitSshKey != "" {
			return errors.New("git ssh key and git ssh key file are mutually exclusive")
		}
		key, err := os.ReadFile(config.GitSshKeyFile)
		if err != nil {
			return fmt.Errorf("failed to read git ssh key file: %w", err)
		}

		config.GitSshKey = string(key)
	}

	return nil
}

func loadConfig() (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()

	v.AddConfigPath(".")
	v.SetConfigType("yml")

	v.SetDefault("port", 8080)
	v.SetDefault("encryptionEnabled", true)

	if err := v.ReadInConfig(); err != nil {
		if errors.Is(err, viper.ConfigFileNotFoundError{}) {
			slog.Debug("config file not found")
		} else {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	config := Config{}
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := checkConfig(config); err != nil {
		return nil, fmt.Errorf("failed to check config: %w", err)
	}

	if config.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	if config.DryRun {
		slog.Info("dry run mode enabled")
	}

	return &config, nil
}
