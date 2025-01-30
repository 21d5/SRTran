// Copyright (c) 2025, soup and the SRTran contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Backend string `toml:"backend"`
	Model   string `toml:"model"`
	APIKey  string `toml:"api_key"`
	BaseURL string `toml:"base_url"`
	RPM     int    `toml:"rpm"`
}

// configPaths returns a list of paths to check for config files
func configPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Warn().Err(err).Msg("could not get user home directory")
		home = ""
	}

	return []string{
		"config.toml",  // current directory
		".srtran.toml", // hidden in current directory
		filepath.Join(home, ".config/srtran/config.toml"), // XDG config home
		filepath.Join(home, ".srtran.toml"),               // hidden in home directory
	}
}

// LoadConfig loads configuration from environment variables and config files
func LoadConfig(configFile string) (*Config, error) {
	config := &Config{}

	// If config file is specified explicitly
	if configFile != "" {
		if _, err := toml.DecodeFile(configFile, config); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
		return config, nil
	}

	// Try default config paths
	for _, path := range configPaths() {
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, config); err == nil {
				log.Debug().Str("path", path).Msg("loaded config file")
				break
			}
		}
	}

	// Environment variables override config file
	if apiKey := os.Getenv("GOOGLE_AI_API_KEY"); apiKey != "" {
		config.Backend = "googleai"
		config.APIKey = apiKey
		if model := os.Getenv("GOOGLE_AI_MODEL"); model != "" {
			config.Model = model
		}
		if rpm := os.Getenv("GOOGLE_AI_RPM"); rpm != "" {
			if val, err := strconv.Atoi(rpm); err == nil {
				config.RPM = val
			}
		}
	} else if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		config.Backend = "openrouter"
		config.APIKey = apiKey
		if model := os.Getenv("OPENROUTER_MODEL"); model != "" {
			config.Model = model
		}
		if rpm := os.Getenv("OPENROUTER_RPM"); rpm != "" {
			if val, err := strconv.Atoi(rpm); err == nil {
				config.RPM = val
			}
		}
	} else if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		config.Backend = "openai"
		config.APIKey = apiKey
		if model := os.Getenv("OPENAI_MODEL"); model != "" {
			config.Model = model
		}
		if rpm := os.Getenv("OPENAI_RPM"); rpm != "" {
			if val, err := strconv.Atoi(rpm); err == nil {
				config.RPM = val
			}
		}
	} else if apiKey := os.Getenv("LMSTUDIO_API_KEY"); apiKey != "" {
		config.Backend = "lmstudio"
		config.APIKey = apiKey
		if baseURL := os.Getenv("LMSTUDIO_BASE_URL"); baseURL != "" {
			config.BaseURL = baseURL
		}
		if model := os.Getenv("LMSTUDIO_MODEL"); model != "" {
			config.Model = model
		}
		if rpm := os.Getenv("LMSTUDIO_RPM"); rpm != "" {
			if val, err := strconv.Atoi(rpm); err == nil {
				config.RPM = val
			}
		}
	}

	return config, nil
}
