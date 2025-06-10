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
	go l.serveConnection(conn)
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
