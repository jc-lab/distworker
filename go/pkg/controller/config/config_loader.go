// distworker
// Copyright (C) 2025 JC-Lab
//
// SPDX-License-Identifier: AGPL-3.0-only
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package config

import (
	"fmt"
	"github.com/go-viper/mapstructure/v2"
	"github.com/jc-lab/distworker/go/internal/storage"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Set defaults
	setDefaults(&config)

	// Validate config
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func ReadDynamicField(input interface{}, target interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:  target,
		TagName: "yaml",
	})
	if err != nil {
		return err
	}
	if err := decoder.Decode(input); err != nil {
		return err
	}

	return nil
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	// Server defaults
	if config.Server.API.Port == 0 {
		config.Server.API.Port = 8080
	}
	if config.Server.Worker.Port == 0 {
		config.Server.Worker.Port = 8081
	}

	// Storage defaults
	if config.Storage.Type == "" {
		config.Storage.Type = "local"
		config.Storage.Other["local"] = &storage.LocalConfig{
			Path: "./files",
		}
	}

	// Provisioner defaults
	for name, settings := range config.Provisioner {
		if settings.Autoscale.IdleTimeout == 0 {
			settings.Autoscale.IdleTimeout = 1 * time.Hour
		}
		if settings.Autoscale.Max == 0 {
			settings.Autoscale.Max = 5
		}
		if settings.Autoscale.UpLatency == 0 {
			settings.Autoscale.UpLatency = time.Second * 3
		}
		if settings.Worker.MaxTaskDuration == 0 {
			settings.Worker.MaxTaskDuration = 1 * time.Hour
		}
		config.Provisioner[name] = settings
	}
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Validate server ID
	if config.Server.Id == "" {
		return fmt.Errorf("server.id is required")
	}

	// Validate ports
	if config.Server.API.Port < 1 || config.Server.API.Port > 65535 {
		return fmt.Errorf("server.api.port must be between 1 and 65535")
	}
	if config.Server.Worker.Port < 1 || config.Server.Worker.Port > 65535 {
		return fmt.Errorf("server.worker.port must be between 1 and 65535")
	}
	if config.Server.API.Port == config.Server.Worker.Port {
		return fmt.Errorf("server.api.port and server.worker.port must be different")
	}

	// Validate database
	if config.Database.Type == "" {
		return fmt.Errorf("database.type is required")
	}

	// Validate storage
	if config.Storage.Type == "" {
		return fmt.Errorf("storage.type is required")
	}

	// Validate provisioners
	for name, settings := range config.Provisioner {
		if settings.Type == "url" && settings.URL == "" {
			return fmt.Errorf("provisioner.%s.url is required when type is 'url'", name)
		}
		if settings.Type == "none" && settings.Token == "" {
			return fmt.Errorf("provisioner.%s.token is required when type is 'none'", name)
		}
		//if len(settings.QueuePatterns) == 0 {
		//	return fmt.Errorf("provisioner.%s.queue_patterns is required", name)
		//}
		if settings.Autoscale.Max <= 0 {
			return fmt.Errorf("provisioner.%s.autoscale.max must be greater than 0", name)
		}
		if settings.Autoscale.Min < 0 {
			return fmt.Errorf("provisioner.%s.autoscale.min must be greater than or equal to 0", name)
		}
		if settings.Autoscale.Min > settings.Autoscale.Max {
			return fmt.Errorf("provisioner.%s.autoscale.min must be less than or equal to max", name)
		}
	}

	return nil
}
