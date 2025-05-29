package controller

import (
	"gopkg.in/yaml.v3"
	"time"
)

// Config represents the controller configuration
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Database    DatabaseConfig    `yaml:"database"`
	Storage     StorageConfig     `yaml:"storage"`
	Queues      []QueueConfig     `yaml:"queues"`
	Provisioner ProvisionerConfig `yaml:"provisioner"`
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
	Port int `yaml:"port"`
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

// ProvisionerSettings represents individual provisioner settings
type ProvisionerSettings struct {
	Type          string                 `yaml:"type"`
	URL           string                 `yaml:"url,omitempty"`
	Token         string                 `yaml:"token,omitempty"`
	QueuePatterns []string               `yaml:"queue_patterns"`
	Worker        WorkerSettings         `yaml:"worker"`
	Parameters    map[string]interface{} `yaml:"parameters,omitempty"`
}

// WorkerSettings represents worker settings for provisioner
type WorkerSettings struct {
	Min             int           `yaml:"min"`
	Max             int           `yaml:"max"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	MaxTaskDuration time.Duration `yaml:"max_task_duration"`
}
