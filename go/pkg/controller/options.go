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
	"github.com/jc-lab/distworker/go/pkg/controller/config"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	storage2 "github.com/jc-lab/distworker/go/pkg/controller/storage"
	"go.uber.org/zap"
)

type Option func(config *config.Config, server *Server) error

func WithDatabase(db database.Database) Option {
	return func(config *config.Config, server *Server) error {
		server.db = db
		return nil
	}
}

func WithStorage(stor storage2.Storage) Option {
	return func(config *config.Config, server *Server) error {
		server.storage = stor
		return nil
	}
}

func WithLogger(rootLogger *zap.Logger) Option {
	return func(config *config.Config, server *Server) error {
		server.rootLogger = rootLogger
		return nil
	}
}
