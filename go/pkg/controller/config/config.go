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
	"github.com/jc-lab/distworker/go/pkg/models"
	"gopkg.in/yaml.v3"
	"time"
)

// Config represents the controller configuration
type Config struct {
	Server            ServerConfig             `yaml:"server"`
	Database          DatabaseConfig           `yaml:"database"`
	Storage           StorageConfig            `yaml:"storage"`
	Queues            []QueueConfig            `yaml:"queues"`
	Provisioner       ProvisionerConfig        `yaml:"provisioner"`
	ControllerSetting models.ControllerSetting `yaml:"controller_setting"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Id     string       `yaml:"id"`
	API    APIConfig    `yaml:"api"`
	Worker WorkerConfig `yaml:"worker"`
}

// APIConfig represents API server configuration
type APIConfig struct {
	Port int `yaml:"port"`
}

// WorkerConfig represents worker server configuration
type WorkerConfig struct {
	Port              int    `yaml:"port"`
	AccessibleBaseUrl string `yaml:"accessible_base_url"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Type  string                 `yaml:"type"`
	Other map[string]interface{} `yaml:"-"`
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
	Type  string                 `yaml:"type"`
	Other map[string]interface{} `yaml:"-"`

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
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
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
	Type ProvisionerType `yaml:"type"`

	// Token is available in Type is ProvisionerTypeNone
	Token string `yaml:"token,omitempty"`

	// URL is available in Type is ProvisionerTypeUrl
	URL string `yaml:"url,omitempty"`

	// Command is available in Type is ProvisionerTypeCommand
	Command *struct {
		WorkingDirectory string            `yaml:"working_directory"`
		Command          []string          `yaml:"command"`
		Envs             map[string]string `yaml:"envs"`
		// for python
		Venv string `yaml:"venv"`
	} `yaml:"command,omitempty"`

	Queues       []string               `yaml:"queues"`
	Worker       WorkerSettings         `yaml:"worker"`
	ReadyTimeout time.Duration          `yaml:"ready_timeout"`
	Autoscale    AutoScaleSettings      `yaml:"autoscale"`
	Parameters   map[string]interface{} `yaml:"parameters,omitempty"`
}

// WorkerSettings represents worker settings for provisioner
type WorkerSettings struct {
	MaxTaskDuration time.Duration `yaml:"max_task_duration"`
}

// AutoScaleSettings represents worker settings for provisioner
type AutoScaleSettings struct {
	Min         int           `yaml:"min"`
	Max         int           `yaml:"max"`
	IdleTimeout time.Duration `yaml:"idle_timeout"`
	UpLatency   time.Duration `yaml:"up_latency"`
}
