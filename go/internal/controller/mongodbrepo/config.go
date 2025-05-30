package mongodbrepo

import (
	"fmt"
)

const DatabaseType = "mongodb"

// MongoDBConfig implements DatabaseConfig for MongoDB
type MongoDBConfig struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
}

// Validate validates the configuration
func (c *MongoDBConfig) Validate() error {
	if c.URI == "" {
		return fmt.Errorf("MongoDB URI is required")
	}
	if c.Database == "" {
		return fmt.Errorf("MongoDB database name is required")
	}
	return nil
}
