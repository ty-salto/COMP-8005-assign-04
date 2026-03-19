package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"assign4/internal/constants"
	"assign4/internal/messages"
)

// Controller Main
func main() {
	startTotal := time.Now()

	shadowPath, username, port, hbSec, chunkSize, cpInterval, err := parseArgs()
	if err != nil {
		usage(err)
		os.Exit(2)
	}

	startParse := time.Now()
	fullHash, err := loadShadowHash(shadowPath, username)
	if err != nil {
		usage(err)
		os.Exit(2)
	}
	alg, err := validateHash(fullHash)
	if err != nil {
		usage(err)
		os.Exit(2)
	}
	parseDur := time.Since(startParse)

	// start_listener
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		usage(fmt.Errorf("listen failed: %w", err))
		os.Exit(2)
	}
	defer ln.Close()

	fmt.Printf("[controller] listening on :%d\n", port)

	allocator := NewChunkAllocator(chunkSize)

	// worker tracking (for cleanup)
	var (
		workersMu sync.Mutex
		workers   = make(map[uint64]*WorkerState)
		closing   atomic.Bool
	)

	// final result (first FOUND or first ERROR). NOT_FOUND never ends the controller.
	foundCh := make(chan messages.ResultMsg, 1)
	errCh := make(chan error, 1)
	var finalSent atomic.Bool

	var firstJobSentNs atomic.Int64
	//startDispatch := time.Now()

	// --- chunk assignment metrics ---
	var chunkAssignTotalNs atomic.Int64 // total time spent sending chunk/JOB messages
	var chunksAssigned atomic.Uint64    // number of successful chunk/JOB sends

	// accept workers forever; each worker gets chunks sequentially.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if closing.Load() {
					return
				}
				errCh <- fmt.Errorf("accept failed: %w", err)
				return
			}

			w := newWorkerState(conn)
			fmt.Printf("[controller] %s connected\n", w)

			// REGISTER/ACK handshake (must happen before StartReceiver)
			if err := w.registerHandshake(); err != nil {
				_ = conn.Close()
				errCh <- err
				return
			}

			w.startReceiver()
			//go StartHeartbeatSender(w.Conn, hbSec, w.HbStop)
			go StartHeartbeatSender(w, hbSec, w.HbStop, allocator)

			workersMu.Lock()
			workers[w.ID] = w
			workersMu.Unlock()

			// Send this worker their first chunk immediately.
			start, end := allocator.Next()
			w.updateRange(start,end)
			job := messages.JobMsg{
				Type:       messages.JOB,
				Username:   username,
				FullHash:   fullHash,
				Alg:        alg,
				Charset:    constants.LegalCharset79,
				RangeStart: w.CurrentStart,
				RangeEnd:   w.CurrentEnd,
				CPInterval: cpInterval,
			}

			// --- time initial chunk assignment send ---
			t0 := time.Now()
			if err := messages.Send(w.Conn, job); err != nil {
				_ = w.Conn.Close()
				errCh <- fmt.Errorf("send JOB to %s failed: %w", w, err)
				return
			}
			chunkAssignTotalNs.Add(time.Since(t0).Nanoseconds())
			chunksAssigned.Add(1)


			firstJobSentNs.CompareAndSwap(0, time.Now().UnixNano())

			// Worker loop: whenever it reports NOT_FOUND, send the next chunk.
			go func(w *WorkerState) {
				for {
					select {
					case hb := <-w.Inbox.HbRes:
						fmt.Printf("[HB %s] tested=%d delta=%d done=%v found=%v\n",
							w, hb.Tested, hb.DeltaTested, hb.Done, hb.Found)

					case cp := <-w.Inbox.Checkpoint:
						w.updateCurrentIndex(cp.CpIdx)
						fmt.Printf("[Controller] Worker-%d: checkpoint index -> %d\n", w.ID, cp.CpIdx)
					
					case res := <-w.Inbox.Result:
						if res.Status == "FOUND" || res.Status == "ERROR" {
							if finalSent.CompareAndSwap(false, true) {
								select {
								case foundCh <- res:
									fmt.Printf("[Controller] Found Password (Worker[ID]=%d)\n",w.ID)	

								default:
								}
							}
							return
						}

						// NOT_FOUND => give more work
						if finalSent.Load() {
							return
						}
						start, end := allocator.Next()
						w.updateRange(start, end)

						job := messages.JobMsg{
							Type:       messages.JOB,
							Username:   username,
							FullHash:   fullHash,
							Alg:        alg,
							Charset:    constants.LegalCharset79,
							RangeStart: start,
							RangeEnd:   end,
							CPInterval: cpInterval,
						}
						fmt.Printf("[Controller] Worker[ID]=%d Start=%d End=%d\n", w.ID, start, end)

						// --- time subsequent chunk assignment send ---
						t0 := time.Now()
						if err := messages.Send(w.Conn, job); err != nil {
							if finalSent.CompareAndSwap(false, true) {
								foundCh <- messages.ResultMsg{
									Type:   messages.RESULT,
									Status: "ERROR",
									Error:  fmt.Sprintf("send JOB to %s failed: %v", w, err),
								}
							}
							return
						}
						chunkAssignTotalNs.Add(time.Since(t0).Nanoseconds())
						chunksAssigned.Add(1)

					case <-w.Inbox.Errors:
						fmt.Printf("[controller] worker %d disconnected - Checkpoint: start:%d end:%d\n", w.ID, w.CurrentIndex, w.CurrentEnd)

						workersMu.Lock()
						delete(workers, w.ID)
						workersMu.Unlock()

						if !finalSent.Load() {
							allocator.Reinsert(w.CurrentIndex, w.CurrentEnd)
						}

						_ = w.Conn.Close()
						return
					}
				}
			}(w)
		}
	}()

	// job_dispatch_ms: time until the FIRST chunk is sent to the FIRST worker.
	//dispatchDur := time.Duration(0)
	// if ns := firstJobSentNs.Load(); ns != 0 {
	// 	//dispatchDur = time.Duration(ns - startDispatch.UnixNano())
	// }

	startReturn := time.Now()
	var res messages.ResultMsg
	select {
	case res = <-foundCh:
		closing.Store(true)
		_ = ln.Close()

		// 1) snapshot workers
		workersMu.Lock()
		ws := make([]*WorkerState, 0, len(workers))
		for _, w := range workers {
			ws = append(ws, w)
		}
		workersMu.Unlock()

		// 2) stop heartbeats (ok to do early)
		for _, w := range ws {
			close(w.HbStop)
		}

		// 3) send STOP to everyone
		for _, w := range ws {
			fmt.Printf("[Controller] Stop signal sent (Worker[ID]=%d)\n",w.ID)
			
			_ = messages.Send(w.Conn, messages.StopMsg{Type: messages.STOP})
		}

		// 4) wait for all STOP_ACKs (or disconnect/timeout)
		var wg sync.WaitGroup
		wg.Add(len(ws))

		for _, w := range ws {
			go func(w *WorkerState) {
				defer wg.Done()

				select {
				case <-w.Inbox.StopAck:
					fmt.Printf("[controller] %s STOP_ACK received\n", w)

				case err := <-w.Inbox.Errors:
					// during shutdown, a closed-conn read is acceptable
					if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
						fmt.Printf("[controller] %s closed (expected)\n", w)
					} else {
						fmt.Printf("[controller] %s disconnected while stopping: %v\n", w, err)
					}

				case <-time.After(30 * time.Second): // timeout set to 30 sec
					fmt.Printf("[controller] %s STOP_ACK timeout\n", w)
				}

				// 5) now cleanup close
				_ = w.Conn.Close()
			}(w)
		}

		wg.Wait()

	case err := <-errCh:
		closing.Store(true)
		_ = ln.Close()
		usage(err)
		os.Exit(2)
	}
	returnDur := time.Since(startReturn)

	if res.Type != messages.RESULT {
		usage(fmt.Errorf("protocol error: expected RESULT"))
		os.Exit(2)
	}

	// report
	fmt.Println("----- FINAL RESULT -----")
	fmt.Printf("status: %s\n", res.Status)
	if res.Status == "FOUND" {
		fmt.Printf("password: %s\n", res.Password)
	}
	if res.Status == "ERROR" {
		fmt.Printf("error: %s\n", res.Error)
	}

	// --- compute chunk assignment totals/avg for reporting ---
	totalAssignMs := float64(chunkAssignTotalNs.Load()) / 1e6
	avgAssignMs := 0.0
	if n := chunksAssigned.Load(); n > 0 {
		avgAssignMs = totalAssignMs / float64(n)
	}

	fmt.Println("----- TIMING -----")
	fmt.Printf("controller_parse_ms: %.3f\n", float64(parseDur.Microseconds())/1000.0)
	fmt.Printf("chunk_assign_total_ms: %.3f\n", totalAssignMs)
	fmt.Printf("chunk_assign_avg_ms: %.3f\n", avgAssignMs)
	fmt.Printf("worker_compute_ms: %.3f\n", float64(res.WorkerComputeNs)/1e6)
	fmt.Printf("result_return_ms: %.3f\n", float64(returnDur.Microseconds())/1000.0)
	fmt.Printf("total_end_to_end_ms: %.3f\n", float64(time.Since(startTotal).Microseconds())/1000.0)
}

// display_usage
func usage(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	fmt.Fprintln(os.Stderr, "usage: controller -f <shadow file> -u <username> -p <port> -b <heartbeat_seconds> -c <chunk_size>")
}