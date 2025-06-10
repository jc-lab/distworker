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
