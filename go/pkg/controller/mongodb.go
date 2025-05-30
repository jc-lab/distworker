package controller

import (
	"fmt"
	mongodbrepo2 "github.com/jc-lab/distworker/go/internal/controller/mongodbrepo"
	"github.com/jc-lab/distworker/go/pkg/controller/config"
)

func useMongodb(cfg *config.Config, server *Server) error {
	dbConfig := &mongodbrepo2.MongoDBConfig{}

	if err := config.ReadDynamicField(cfg.Database.Other[mongodbrepo2.DatabaseType], dbConfig); err != nil {
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
