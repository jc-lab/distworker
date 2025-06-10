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
