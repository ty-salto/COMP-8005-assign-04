package main

import (
	"bufio"
	"fmt"
	"net"
	"sync/atomic"

	"assign4/internal/messages"
)

type WorkerState struct {
	ID     uint64
	Conn   net.Conn
	Reader *bufio.Reader
	Inbox  *Inbox
	CurrentStart uint64
	CurrentEnd	 uint64
	CurrentIndex uint64
	
	HbStop chan struct{}
}

var nextWorkerID uint64

func newWorkerState(conn net.Conn) *WorkerState {
	id := atomic.AddUint64(&nextWorkerID, 1)
	r := bufio.NewReader(conn)
	inbox := MakeInbox()

	return &WorkerState{
		ID:     		id,
		Conn:   		conn,
		Reader: 		r,
		Inbox:  		inbox,
		CurrentStart:	0,
		CurrentEnd:		0,
		CurrentIndex: 	0,
		HbStop: make(chan struct{}),
	}
}

func (w *WorkerState) String() string {
	return fmt.Sprintf("worker-%d", w.ID)
}

func (w *WorkerState) updateRange(start, end uint64) {
	w.CurrentStart = start;
	w.CurrentEnd = end;
	w.CurrentIndex = start;
}

func (w *WorkerState)updateCurrentIndex (currIdx uint64) {
	w.CurrentIndex = currIdx;
}


// registerHandshake expects REGISTER then sends ACK OK.
func (w *WorkerState) registerHandshake() error {
	var reg messages.RegisterMsg
	if err := messages.RecvLine(w.Reader, &reg); err != nil {
		return fmt.Errorf("read REGISTER failed: %w", err)
	}
	if reg.Type != messages.REGISTER {
		_ = messages.Send(w.Conn, messages.AckMsg{Type: messages.ACK, Status: "ERROR", Error: "expected REGISTER"})
		return fmt.Errorf("protocol error: expected REGISTER")
	}
	return messages.Send(w.Conn, messages.AckMsg{Type: messages.ACK, Status: "OK"})
}

// startReceiver must be called AFTER the initial REGISTER/ACK handshake,
func (w *WorkerState) startReceiver() {
	StartReceiver(w.Reader, w.Inbox)
}