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
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"log"
	"sync"
	"time"
)

// connection represents a single worker WebSocket connection
type connection struct {
	l *Listener

	mu sync.RWMutex

	workerId    string
	conn        *websocket.Conn
	session     *models2.WorkerSession
	lastPing    time.Time
	lastPong    time.Time
	sendChannel chan *send

	// rtt ns
	rtt int64
}

var errTimeout = errors.New("timeout")
var errSendFull = errors.New("worker send channel is full")

const PingInterval = 5 * time.Second
const ReadTimeout = PingInterval * 3
const SendTimeout = 10 * time.Second
const HeartbeatTimeout = 60 * time.Second

type send struct {
	data   []byte
	result chan error
}

func newConnection(l *Listener, conn *websocket.Conn) *connection {
	wsConn := &connection{
		l:           l,
		conn:        conn,
		lastPing:    time.Now(),
		lastPong:    time.Now(),
		sendChannel: make(chan *send, 10),
		rtt:         -1,
	}
	return wsConn
}

func (conn *connection) GetWorkerId() string {
	return conn.workerId
}

func (conn *connection) GetSession() *models2.WorkerSession {
	return conn.session
}

func (conn *connection) Close() error {
	return conn.conn.Close()
}

func (conn *connection) GetRtt() time.Duration {
	conn.mu.RLock()
	defer conn.mu.RUnlock()
	return time.Nanosecond * time.Duration(conn.rtt)
}

func (conn *connection) GetLastPong() time.Time {
	conn.mu.RLock()
	defer conn.mu.RUnlock()
	return conn.lastPong
}

func (conn *connection) SendPayload(message *protocol2.WebSocketMessage) error {
	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	s := &send{
		data:   data,
		result: make(chan error),
	}
	select {
	case conn.sendChannel <- s:
	default:
		return errSendFull
	}
	select {
	case err = <-s.result:
		return err
	case <-time.After(3 * time.Second):
		return errTimeout
	}
}

// register handles the initial worker registration
func (conn *connection) register() error {
	// Read registration message
	_ = conn.conn.SetReadDeadline(time.Now().Add(ReadTimeout))
	messageType, data, err := conn.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read registration message: %w", err)
	}

	if messageType != websocket.BinaryMessage {
		return fmt.Errorf("expected binary message for registration")
	}

	// Parse WebSocket message
	var wsMsg protocol2.WebSocketMessage
	if err := proto.Unmarshal(data, &wsMsg); err != nil {
		return fmt.Errorf("failed to unmarshal registration message: %w", err)
	}

	if wsMsg.Type != protocol2.MessageType_MESSAGE_TYPE_WORKER_REGISTER {
		return fmt.Errorf("expected worker registration message, got: %v", wsMsg.Type)
	}

	register := wsMsg.GetWorkerRegister()
	if register == nil {
		return fmt.Errorf("worker register payload is nil")
	}

	session, response, err := conn.l.manager.HandleRegister(conn, register)
	if err != nil {
		return err
	}

	conn.workerId = session.WorkerId
	conn.session = session

	return conn.SendPayload(&protocol2.WebSocketMessage{
		Type: protocol2.MessageType_MESSAGE_TYPE_WORKER_REGISTER_RESPONSE,
		Payload: &protocol2.WebSocketMessage_WorkerRegisterResponse{
			WorkerRegisterResponse: response,
		},
	})
}

func (conn *connection) sender() {
	pingTicker := time.NewTicker(3 * time.Second)
	defer pingTicker.Stop()

	// Set connection timeouts
	conn.conn.SetPongHandler(func(string) error {
		_ = conn.conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		conn.handlePong()
		return nil
	})

	conn.sendPing()

	for {
		select {
		case s, ok := <-conn.sendChannel:
			if !ok {
				return
			}
			_ = conn.conn.SetWriteDeadline(time.Now().Add(SendTimeout))
			if err := conn.conn.WriteMessage(websocket.BinaryMessage, s.data); err != nil {
				log.Printf("Failed to send message to worker %s: %v", conn.workerId, err)
				s.result <- err
				close(s.result)
				return
			} else {
				s.result <- nil
				close(s.result)
			}
		case <-pingTicker.C:
			conn.sendPing()
		}
	}
}

func (conn *connection) sendPing() {
	// Send ping
	_ = conn.conn.SetWriteDeadline(time.Now().Add(SendTimeout))
	if err := conn.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		return
	}
	conn.mu.Lock()
	conn.lastPing = time.Now()
	conn.mu.Unlock()
}

func (conn *connection) serve() {
	// Handle incoming messages
	for {
		select {
		case <-conn.l.ctx.Done():
			return
		default:
			_ = conn.conn.SetReadDeadline(time.Now().Add(ReadTimeout))
			messageType, data, err := conn.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error for worker %s: %v", conn.workerId, err)
				}
				return
			}

			switch messageType {
			case websocket.BinaryMessage:
				var wsMsg protocol2.WebSocketMessage
				if err := proto.Unmarshal(data, &wsMsg); err != nil {
					log.Printf("Worker[%s] Failed to unmarshal worker message: %v", conn.workerId, err)
				}

				if err := conn.l.manager.HandleWsWorkerMessage(conn, &wsMsg); err != nil {
					log.Printf("Worker[%s] Error handling message: %v", conn.workerId, err)
				}
			}
		}
	}
}

func (conn *connection) handlePong() {
	now := time.Now()
	rtt := now.Sub(conn.lastPing)

	conn.mu.Lock()
	defer conn.mu.Unlock()

	conn.lastPong = now
	conn.rtt = rtt.Nanoseconds()
}
