package controller

import (
	"github.com/jc-lab/distworker/go/pkg/controller/config"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	storage2 "github.com/jc-lab/distworker/go/pkg/controller/storage"
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
