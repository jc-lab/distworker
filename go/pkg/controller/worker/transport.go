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

package worker

import (
	protocol2 "github.com/jc-lab/distworker/go/internal/protocol"
	models2 "github.com/jc-lab/distworker/go/pkg/models"
	"io"
	"time"
)

type Listener interface {
}

type Connection interface {
	io.Closer

	GetWorkerId() string
	GetSession() *models2.WorkerSession

	// SendPayload must check sent successfully
	SendPayload(message *protocol2.WebSocketMessage) error

	GetRtt() time.Duration
	GetLastPong() time.Time
}
