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

package main

import (
	"flag"
	controller2 "github.com/jc-lab/distworker/go/pkg/controller"
	config2 "github.com/jc-lab/distworker/go/pkg/controller/config"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var k = koanf.New(".")

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", os.Getenv("DISTWORKER_CONFIG"), "DISTWORKER_CONFIG path to config file")
	flag.Parse()

	if configPath != "" {
		if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
			log.Fatalf("Error loading config from file: %v", err)
		}
	}

	if err := k.Load(env.Provider("DISTWORKER_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "DISTWORKER_")), "_", ".", -1)
	}), nil); err != nil {
		log.Fatalf("Error loading config from env: %v", err)
	}

	config := &config2.Config{}
	if err := config.Load(k); err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	log.Printf("Starting distworker controller (server Id: %s)", config.Server.Id)

	// Create and start server
	server, err := controller2.NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal")
		server.Stop()
	}()

	// Start server (this blocks until shutdown)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server shutdown complete")
}
