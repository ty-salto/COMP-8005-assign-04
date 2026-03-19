package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"assign4/internal/messages"
)

//Worker Main
func main() {
	host, port, numT, err := parseArgs()
	if err != nil {
		usage(err)
		os.Exit(2)
	}


	//connect_to_controller
	fmt.Println("[worker] Connecting...")
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		usage(fmt.Errorf("controller unreachable: %w", err))
		os.Exit(2)
	}
	fmt.Println("[worker] Connected")
	defer conn.Close()

	r := bufio.NewReader(conn)

	// receiving and writingmessages
    inbox := MakeWorkerInbox()
    StartWorkerReceiver(r, inbox)

    sendQ := make(chan any, 64)
    writerDone := make(chan struct{})
    writerErr := make(chan error, 1)

    go func() {
        defer close(writerDone)
        for msg := range sendQ {
            if err := messages.Send(conn, msg); err != nil {
                select { case writerErr <- err: default: }
                return
            }
        }
    }()

	// register_worker
	sendQ <- messages.RegisterMsg{
        Type:   messages.REGISTER,
        Worker: hostnameOr("worker"),
    }

    select {
    case ack := <-inbox.Ack:
        if ack.Type != messages.ACK || ack.Status != "OK" {
            close(sendQ); <-writerDone
            return
        }
    case err := <-inbox.Err:
        _ = err
        close(sendQ); <-writerDone
        return
    case err := <-writerErr:
        _ = err
        close(sendQ); <-writerDone
        return
    }

	// --- Worker lives forever and keeps waiting for jobs ---
	var (
		cracking    bool
		job         messages.JobMsg
		crackDoneCh chan *messages.ResultMsg

		lastTotal uint64
	)

	for {
		if !cracking {
			// IDLE: wait for a JOB (but still answer heartbeat requests)
			select {
			case hb := <-inbox.HbReq:
				// Idle progress = 0. (Controller should treat worker as idle anyway.)
				total := atomic.LoadUint64(&totalTested)
				delta := total - lastTotal
				lastTotal = total

				sendQ <- messages.HeartbeatRes{
					Type:        messages.HEARTBEAT_RES,
					Seq:         hb.Seq,
					DeltaTested: delta,
					Tested:      total,
					Done:        true,  // idle == not currently cracking
					Found:       false, // idle == not found (for now)
				}

			case job = <-inbox.Job:
				// got a job: validate then start cracking
				if err := validateJob(&job); err != nil {
					sendQ <- messages.ResultMsg{Type: messages.RESULT, Status: "ERROR", Error: err.Error()}
					// stay alive; keep waiting for next job
					continue
				}

				fmt.Printf("[Worker] Job Received - Start=%d End=%d\n",job.RangeStart, job.RangeEnd)

				// reset per-job progress counters
				atomic.StoreUint64(&totalTested, 0)
				crackDone.Store(0)
				crackFound.Store(0)
				lastTotal = 0

				crackDoneCh = make(chan *messages.ResultMsg, 1)
				cracking = true

				go func(j messages.JobMsg) {
					startCompute := time.Now()
					res := crack(&j, numT, sendQ)
					res.WorkerComputeNs = time.Since(startCompute).Nanoseconds()
					crackDoneCh <- res
				}(job)

			case <-inbox.Stop:
				fmt.Println("[Worker] Stopping threads")
				stopThreads()
				// enqueue ack
				sendQ <- messages.StopAckMsg{Type: messages.STOP_ACK}

				// close queue so writer finishes after flushing pending msgs
				close(sendQ)

				// wait for writer to finish (or timeout)
				select {
				case <-writerDone:
				case <-time.After(1 * time.Second):
					fmt.Println("[Worker] writer flush timeout")
				}

				return

			case err := <-inbox.Err:
				_ = err
				close(sendQ)
				<-writerDone
				return

			case err := <-writerErr:
				_ = err
				close(sendQ)
				<-writerDone
				return
			}

			continue
		}

		// CRACKING: handle heartbeats + final result
		select {
		case hb := <-inbox.HbReq:
			total := atomic.LoadUint64(&totalTested)
			delta := total - lastTotal
			lastTotal = total

			fmt.Println("[HB] responded...")

			sendQ <- messages.HeartbeatRes{
				Type:        messages.HEARTBEAT_RES,
				Seq:         hb.Seq,
				DeltaTested: delta,
				Tested:      total,
				Done:        crackDone.Load() != 0,
				Found:       crackFound.Load() != 0,
			}

		case res := <-crackDoneCh:
			// Send result, then go back to idle and wait for next job
			sendQ <- res
			cracking = false
			crackDoneCh = nil
			lastTotal = 0

		case err := <-inbox.Err:
			_ = err
			close(sendQ)
			<-writerDone
			return

		case err := <-writerErr:
			_ = err
			close(sendQ)
			<-writerDone
			return
		}
	}
}

// consider deleting
func sendError(conn net.Conn, err error) {
	_ = messages.Send(conn, &messages.ResultMsg{
		Type:   messages.RESULT,
		Status: "ERROR",
		Error:  err.Error(),
	})
}

func usage(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	fmt.Fprintln(os.Stderr, "usage: worker -c <controller_host> -p <port> -t <threads>")
}

func hostnameOr(fallback string) string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return fallback
	}
	return h
}
