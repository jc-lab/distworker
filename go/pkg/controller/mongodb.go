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

package controller

import (
	"fmt"
	mongodbrepo2 "github.com/jc-lab/distworker/go/internal/controller/mongodbrepo"
	"github.com/jc-lab/distworker/go/pkg/controller/config"
)

func useMongodb(cfg *config.Config, server *Server) error {
	dbConfig := &mongodbrepo2.MongoDBConfig{}

	if err := cfg.ReadDynamicField("database."+mongodbrepo2.DatabaseType, dbConfig); err != nil {
		return err
	}

	// Validate database cfg
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("invalid database cfg: %w", err)
	}

	// Connect to database
	db, err := mongodbrepo2.NewDB(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	server.db = db

	return nil
}
