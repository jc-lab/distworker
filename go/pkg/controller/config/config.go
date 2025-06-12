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
	"github.com/jc-lab/distworker/go/pkg/models"
	"github.com/knadh/koanf/v2"
	"gopkg.in/yaml.v3"
	"time"
)

// Config represents the controller configuration
type Config struct {
	Server            ServerConfig             `koanf:"server"`
	Database          DatabaseConfig           `koanf:"database"`
	Storage           StorageConfig            `koanf:"storage"`
	Queues            []QueueConfig            `koanf:"queues"`
	Provisioner       ProvisionerConfig        `koanf:"provisioner"`
	ControllerSetting models.ControllerSetting `koanf:"controller_setting"`

	k *koanf.Koanf
}

func (c *Config) Load(k *koanf.Koanf) error {
	c.k = k
	if err := k.Unmarshal("", c); err != nil {
		return err
	}

	// Set defaults
	setDefaults(c)

	// Validate config
	if err := validateConfig(c); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	return nil
}

func (c *Config) ReadDynamicField(key string, out interface{}) error {
	return c.k.Unmarshal(key, out)
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Id     string       `koanf:"id"`
	API    APIConfig    `koanf:"api"`
	Worker WorkerConfig `koanf:"worker"`
}

// APIConfig represents API server configuration
type APIConfig struct {
	Port   int             `koanf:"port"`
	OpenAi OpenAiApiConfig `koanf:"openai"`
}

type OpenAiApiConfig struct {
	Enabled              bool   `koanf:"enabled"`
	ChatCompletionsQueue string `koanf:"chat_completions_queue"`
	CompletionsQueue     string `koanf:"completions_queue"`
	EmbeddingsQueue      string `koanf:"embeddings_queue"`
}

// WorkerConfig represents worker server configuration
type WorkerConfig struct {
	Port              int    `koanf:"port"`
	AccessibleBaseUrl string `koanf:"accessible_base_url"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Type  string                 `koanf:"type"`
	Other map[string]interface{} `koanf:"-"`
}

func (c *DatabaseConfig) UnmarshalYAML(value *yaml.Node) error {
	// 모든 필드를 임시 맵에 언마샬
	var rawMap map[string]interface{}
	if err := value.Decode(&rawMap); err != nil {
		return err
	}

	// type 필드 처리
	if typeVal, ok := rawMap["type"]; ok {
		c.Type = typeVal.(string)
	}

	// Other 슬라이스 초기화
	c.Other = make(map[string]interface{})

	// type을 제외한 다른 모든 필드를 Other에 추가
	for key, val := range rawMap {
		if key != "type" {
			c.Other[key] = val
		}
	}

	return nil
}

// StorageConfig represents storage configuration
type StorageConfig struct {
	Type  string                 `koanf:"type"`
	Other map[string]interface{} `koanf:"-"`

	// s3
	// local
}

func (c *StorageConfig) UnmarshalYAML(value *yaml.Node) error {
	// 모든 필드를 임시 맵에 언마샬
	var rawMap map[string]interface{}
	if err := value.Decode(&rawMap); err != nil {
		return err
	}

	// type 필드 처리
	if typeVal, ok := rawMap["type"]; ok {
		c.Type = typeVal.(string)
	}

	// Other 슬라이스 초기화
	c.Other = make(map[string]interface{})

	// type을 제외한 다른 모든 필드를 Other에 추가
	for key, val := range rawMap {
		if key != "type" {
			c.Other[key] = val
		}
	}

	return nil
}

// QueueConfig represents queue configuration
type QueueConfig struct {
	Name        string `koanf:"name"`
	Description string `koanf:"description"`
}

// ProvisionerConfig represents provisioner configuration map
type ProvisionerConfig map[string]ProvisionerSettings

type ProvisionerType string

const (
	ProvisionerTypeNone    ProvisionerType = "none"
	ProvisionerTypeUrl     ProvisionerType = "url"
	ProvisionerTypeCommand ProvisionerType = "command"

	// TODO: separate project & use ProvisionerTypeUrl instead of.
	ProvisionerTypeVast ProvisionerType = "vast"
)

// ProvisionerSettings represents individual provisioner settings
type ProvisionerSettings struct {
	Type ProvisionerType `koanf:"type"`

	// Token is available in Type is ProvisionerTypeNone
	Token string `koanf:"token,omitempty"`

	// URL is available in Type is ProvisionerTypeUrl
	URL string `koanf:"url,omitempty"`

	// Command is available in Type is ProvisionerTypeCommand
	Command *struct {
		WorkingDirectory string            `koanf:"working_directory"`
		Command          []string          `koanf:"command"`
		Envs             map[string]string `koanf:"envs"`
		// for python
		Venv string `koanf:"venv"`
	} `koanf:"command,omitempty"`

	Queues       []string               `koanf:"queues"`
	Worker       WorkerSettings         `koanf:"worker"`
	ReadyTimeout time.Duration          `koanf:"ready_timeout"`
	Autoscale    AutoScaleSettings      `koanf:"autoscale"`
	Parameters   map[string]interface{} `koanf:"parameters,omitempty"`
}

// WorkerSettings represents worker settings for provisioner
type WorkerSettings struct {
	MaxTaskDuration time.Duration `koanf:"max_task_duration"`
}

// AutoScaleSettings represents worker settings for provisioner
type AutoScaleSettings struct {
	Min         int           `koanf:"min"`
	Max         int           `koanf:"max"`
	IdleTimeout time.Duration `koanf:"idle_timeout"`
	UpLatency   time.Duration `koanf:"up_latency"`
}
