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

package websocket

import (
	"context"
	"github.com/gorilla/websocket"
	"github.com/jc-lab/distworker/go/internal/provisioner"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	"github.com/jc-lab/distworker/go/pkg/controller/worker"
	"log"
)

type Listener struct {
	ctx                context.Context
	db                 database.Database
	provisionerManager provisioner.Manager
	manager            *worker.Manager
}

var _ worker.Listener = &Listener{}

func NewListener(
	ctx context.Context,
	db database.Database,
	provisionerManager provisioner.Manager,
	manager *worker.Manager,
) *Listener {
	return &Listener{
		ctx:                ctx,
		db:                 db,
		provisionerManager: provisionerManager,
		manager:            manager,
	}
}

func (l *Listener) HandleConnection(conn *websocket.Conn) {
	l.serveConnection(conn)
}

func (l *Listener) serveConnection(conn *websocket.Conn) {
	defer conn.Close()

	workerConn := newConnection(l, conn)
	defer close(workerConn.sendChannel)

	// Start sender goroutine
	go workerConn.sender()
	if err := workerConn.register(); err != nil {
		log.Printf("Remote[%v] worker register failed: %+v", conn.RemoteAddr().String(), err)
		return
	}

	l.addConnection(workerConn)
	defer l.removeConnection(workerConn)

	workerConn.serve()
}

func (l *Listener) addConnection(conn *connection) {
	log.Printf("Worker[%s] connected", conn.workerId)
}

func (l *Listener) removeConnection(conn *connection) {
	l.manager.RemoveConnection(conn)

	// Update worker session status
	if conn.session != nil {
		l.db.GetWorkerSessionRepository().Delete(context.Background(), conn.workerId)
	}

	log.Printf("Worker[%s] disconnected", conn.workerId)
}
