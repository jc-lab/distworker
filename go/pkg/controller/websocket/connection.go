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
	"fmt"
	"github.com/gorilla/websocket"
	protocol2 "github.com/jc-lab/distworker/go/internal/protocol"
	models2 "github.com/jc-lab/distworker/go/pkg/models"
	"google.golang.org/protobuf/proto"
	"sync"
	"time"
)

// WorkerConnection represents a single worker WebSocket connection
type WorkerConnection struct {
	WorkerId    string
	Conn        *websocket.Conn
	Session     *models2.WorkerSession
	LastPing    time.Time
	LastPong    time.Time
	SendChannel chan []byte
	Done        chan struct{}
	mu          sync.RWMutex
}

func (conn *WorkerConnection) GetWorkerId() string {
	return conn.WorkerId
}

func (conn *WorkerConnection) sendPayload(message *protocol2.WebSocketMessage) error {
	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	select {
	case conn.SendChannel <- data:
		return nil
	default:
		return fmt.Errorf("worker send channel is full")
	}
}
