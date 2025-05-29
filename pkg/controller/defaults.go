package controller

import (
	"fmt"
	"github.com/jc-lab/distworker/internal/controller/mongodbrepo"
	"github.com/jc-lab/distworker/internal/storage"
)

func defaultDatabase(config *Config, server *Server) error {
	switch config.Database.Type {
	case mongodbrepo.DatabaseType:
		return useMongodb(config, server)
	}

	return fmt.Errorf("invalid database type: %s", config.Database.Type)
}

func defaultStorage(config *Config, server *Server) error {
	var err error

	switch config.Storage.Type {
	case storage.TypeLocal:
		localConfig := &storage.LocalConfig{}

		if err := ReadDynamicField(config.Storage.Other[storage.TypeLocal], localConfig); err != nil {
			return err
		}

		server.storage, err = storage.NewLocalStorage(localConfig)

	case storage.TypeS3:
		s3Config := &storage.S3Config{}

		if err := ReadDynamicField(config.Storage.Other[storage.TypeS3], s3Config); err != nil {
			return err
		}

		server.storage, err = storage.NewS3Storage(s3Config)

	default:
		return fmt.Errorf("unsupported storage type: %s", config.Storage.Type)
	}

	return err
}
