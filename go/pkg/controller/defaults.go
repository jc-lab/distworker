package controller

import (
	"fmt"
	"github.com/jc-lab/distworker/go/internal/controller/mongodbrepo"
	storage2 "github.com/jc-lab/distworker/go/internal/storage"
	"github.com/jc-lab/distworker/go/pkg/controller/config"
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

		if err := config.ReadDynamicField(cfg.Storage.Other[storage2.TypeLocal], localConfig); err != nil {
			return err
		}

		server.storage, err = storage2.NewLocalStorage(localConfig)

	case storage2.TypeS3:
		s3Config := &storage2.S3Config{}

		if err := config.ReadDynamicField(cfg.Storage.Other[storage2.TypeS3], s3Config); err != nil {
			return err
		}

		server.storage, err = storage2.NewS3Storage(s3Config)

	default:
		return fmt.Errorf("unsupported storage type: %s", cfg.Storage.Type)
	}

	return err
}
