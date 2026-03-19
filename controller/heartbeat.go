package main

import (
	"fmt"
	//"net"
	"time"

	"assign4/internal/messages"
)

// StartHeartbeatSender sends HEARTBEAT_REQ every hbSec seconds until stop is closed.
// It only WRITES to the connection.
func StartHeartbeatSender(w *WorkerState, hbSec int, stop <-chan struct{}, allocator *ChunkAllocator) {
	if hbSec <= 0 {
		return
	}

	ticker := time.NewTicker(time.Duration(hbSec) * time.Second)
	defer ticker.Stop()

	var seq uint64 = 0

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			seq++
			req := messages.HeartbeatReq{
				Type: messages.HEARTBEAT_REQ,
				Seq:  seq,
			}
			// If send fails, the receiver will usually hit an error shortly after anyway.
			fmt.Printf("[HB worker-%d] request sent \n", w.ID)
			err := messages.Send(w.Conn, req)

			if err != nil {
				// fmt.Printf("[HB worker-%d] %s\n", w.ID, err)
				// allocator.Reinsert(w.CurrentIndex, w.CurrentEnd)
				return
			}
		}
	}
}
