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
	"github.com/jc-lab/distworker/go/internal/controller/mongodbrepo"
	storage2 "github.com/jc-lab/distworker/go/internal/storage"
	"github.com/jc-lab/distworker/go/pkg/controller/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

func defaultDatabase(config *config.Config, server *Server) error {
	switch config.Database.Type {
	case mongodbrepo.DatabaseType:
		return useMongodb(config, server)
	}

	return fmt.Errorf("invalid database type: %s", config.Database.Type)
}

func defaultStorage(cfg *config.Config, server *Server) error {
	var err error

	switch cfg.Storage.Type {
	case storage2.TypeLocal:
		localConfig := &storage2.LocalConfig{}

		if err := cfg.ReadDynamicField("storage."+storage2.TypeLocal, localConfig); err != nil {
			return err
		}

		server.storage, err = storage2.NewLocalStorage(localConfig)

	case storage2.TypeS3:
		s3Config := &storage2.S3Config{}

		if err := cfg.ReadDynamicField("storage."+storage2.TypeS3, s3Config); err != nil {
			return err
		}

		server.storage, err = storage2.NewS3Storage(s3Config)

	default:
		return fmt.Errorf("unsupported storage type: %s", cfg.Storage.Type)
	}

	return err
}

func makeDefaultLogger() *zap.Logger {
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), os.Stdout, zap.InfoLevel)
	return zap.New(core)
}
