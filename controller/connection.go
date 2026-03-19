package main

import (
	"bufio"
	"encoding/json"
	"fmt"

	"assign4/internal/messages"
)

// Inbox channels let your controller react without peeking the socket directly.
type Inbox struct {
	Ack     	chan messages.AckMsg
	Result  	chan messages.ResultMsg
	HbRes   	chan messages.HeartbeatRes
	Checkpoint	chan messages.CheckpointMsg
	StopAck 	chan messages.StopAckMsg
	Errors  	chan error
}

// MakeInbox creates buffered channels to avoid deadlocks if messages arrive fast.
func MakeInbox() *Inbox {
	return &Inbox{
		Ack:    	make(chan messages.AckMsg, 8),
		Result: 	make(chan messages.ResultMsg, 2),
		HbRes:  	make(chan messages.HeartbeatRes, 32),
		Checkpoint: make(chan messages.CheckpointMsg, 32),
		StopAck:  	make(chan messages.StopAckMsg, 5),
		Errors: 	make(chan error, 1),
	}
}

// StartReceiver starts ONE thread that continuously reads messages from conn
func StartReceiver(r *bufio.Reader, inbox *Inbox) {

	go func() {
		for {
			raw, err := messages.RecvRawLine(r)
			if err != nil {
				inbox.Errors <- fmt.Errorf("recv failed: %w", err)
				return
			}

			t, err := messages.PeekType(raw)
			if err != nil {
				inbox.Errors <- fmt.Errorf("peek type failed: %w", err)
				return
			}

			switch t {
			case messages.ACK:
				var m messages.AckMsg
				if err := json.Unmarshal(raw, &m); err != nil {
					inbox.Errors <- fmt.Errorf("bad ACK: %w", err)
					return
				}
				inbox.Ack <- m

			case messages.RESULT:
				var m messages.ResultMsg
				if err := json.Unmarshal(raw, &m); err != nil {
					inbox.Errors <- fmt.Errorf("bad RESULT: %w", err)
					return
				}
				inbox.Result <- m

			case messages.HEARTBEAT_RES:
				var m messages.HeartbeatRes
				if err := json.Unmarshal(raw, &m); err != nil {
					inbox.Errors <- fmt.Errorf("bad HEARTBEAT_RES: %w", err)
					return
				}
				inbox.HbRes <- m

			case messages.CHECK_POINT:
				var m messages.CheckpointMsg
				if err := json.Unmarshal(raw, &m); err != nil {
					inbox.Errors <- fmt.Errorf("bad CHECK_POINT: %w", err)
					return
				}
				inbox.Checkpoint <- m
				
			case messages.STOP_ACK:
				var m messages.StopAckMsg
				if err := json.Unmarshal(raw, &m); err != nil {
					inbox.Errors <- fmt.Errorf("bad HEARTBEAT_RES: %w", err)
					return
				}
				inbox.StopAck <- m

			default:
				inbox.Errors <- fmt.Errorf("unknown message type: %s", t)
				return
			}
		}
	}()
}
