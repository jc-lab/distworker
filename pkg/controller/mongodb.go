package controller

import (
	"fmt"
	"github.com/jc-lab/distworker/internal/controller/mongodbrepo"
)

func useMongodb(config *Config, server *Server) error {
	dbConfig := &mongodbrepo.MongoDBConfig{}

	if err := ReadDynamicField(config.Database.Other[mongodbrepo.DatabaseType], dbConfig); err != nil {
		return err
	}

	// Validate database config
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("invalid database config: %w", err)
	}

	// Connect to database
	db, err := mongodbrepo.NewDB(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	server.db = db

	return nil
}
