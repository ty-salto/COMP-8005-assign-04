package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"assign4/internal/messages"

	"golang.org/x/crypto/bcrypt"
)

var (
	resOnce     sync.Once
	finalRes    *messages.ResultMsg
	crackCtx    context.Context
	crackCancel context.CancelFunc

	totalTested uint64

	// shared atomic cursor for dynamic work claiming
	nextIndex atomic.Uint64

	// Can be change to atomic Bool
	crackDone  atomic.Uint32
	crackFound atomic.Uint32
)

// setFinalResult sets the final result ONCE and cancels cracking so other threads stop quickly.
func setFinalResult(r *messages.ResultMsg) {
	resOnce.Do(func() {
		finalRes = r

		if r.Status == "FOUND" {
			crackFound.Store(1)
		}
		crackDone.Store(1)

		if crackCancel != nil {
			crackCancel()
		}
	})
}

// IndexToCandidateInfinite maps a GLOBAL index to a candidate string
// ordered by increasing length: len=1 block, then len=2 block, etc.
// It does NOT include the empty string.
func IndexToCandidateInfinite(globalIdx uint64, charset []byte) (string, error) {
	if len(charset) == 0 {
		return "", fmt.Errorf("charset must not be empty")
	}
	base := uint64(len(charset))

	length := uint64(1)
	blockSize := base // base^1

	for globalIdx >= blockSize {
		globalIdx -= blockSize
		length++

		// Next blockSize *= base, protect overflow
		if blockSize > ^uint64(0)/base {
			return "", fmt.Errorf("index space overflow while expanding length")
		}
		blockSize *= base
	}

	out := make([]byte, length)
	idx := globalIdx
	for i := int(length) - 1; i >= 0; i-- {
		out[i] = charset[idx%base]
		idx /= base
	}
	return string(out), nil
}

// // splitRangeU64 splits across numT threads.
// func splitRangeU64(total uint64, tid, numT int) (uint64, uint64) {
// 	if numT <= 1 {
// 		return 0, total
// 	}
// 	q := total / uint64(numT)
// 	r := total % uint64(numT)

// 	start := uint64(tid)*q + minU64(uint64(tid), r)
// 	size := q
// 	if uint64(tid) < r {
// 		size++
// 	}
// 	return start, start + size
// }

// func minU64(a, b uint64) uint64 {
// 	if a < b {
// 		return a
// 	}
// 	return b
// }

func stopThreads() {
	if crackCancel != nil {
		crackCancel()
	}
}

// crack processes global indices in [job.RangeStart, job.RangeEnd) using numT threads.
// Threads do NOT know the password length,
func crack(job *messages.JobMsg, numT int, sendQ chan<- any) *messages.ResultMsg {
	if numT <= 0 {
		numT = 1
	}
	if job == nil {
		return &messages.ResultMsg{Type: messages.RESULT, Status: "ERROR", Error: "nil job"}
	}

	start := job.RangeStart
	end := job.RangeEnd

	if start >= end {
		return &messages.ResultMsg{Type: messages.RESULT, Status: "NOT_FOUND"}
	}

	// Reset shared state for each crack call
	resOnce = sync.Once{}
	finalRes = &messages.ResultMsg{Type: messages.RESULT, Status: "NOT_FOUND"}
	atomic.StoreUint64(&totalTested, 0)
	crackDone.Store(0)
	crackFound.Store(0)

	charsetBytes := []byte(job.Charset)
	if len(charsetBytes) == 0 {
		return &messages.ResultMsg{Type: messages.RESULT, Status: "ERROR", Error: "empty charset"}
	}

	crackCtx, crackCancel = context.WithCancel(context.Background())

	nextIndex.Store(start)

	var wg sync.WaitGroup
	for tid := 0; tid < numT; tid++ {

		wg.Go(func() {
			fmt.Printf("[Thread - %d] created...\n",tid)
			crackThread(job, charsetBytes, end, tid, sendQ)
        })
	}

	wg.Wait()

	// ensure canceled to release any waiters and avoid leaks
	if crackCancel != nil {
    crackCancel()
}
	return finalRes
}

func crackThread(job *messages.JobMsg, charset []byte, end uint64, tid int, sendQ chan<- any) {
    ctx := newCryptCtx()

    var localTested uint64
    const flushEvery = 1024

    flush := func() {
        if localTested != 0 {
            atomic.AddUint64(&totalTested, localTested)
            localTested = 0
        }
    }
    defer flush()

	for {
		select {
		case <-crackCtx.Done():
			flush()
			return
		default:
		}

		// claim one unique global index
		g := nextIndex.Add(1) - 1
		if g >= end {
			flush()
			return
		}

		cand, err := IndexToCandidateInfinite(g, charset)
		if err != nil {
			flush()
			setFinalResult(&messages.ResultMsg{
				Type:   messages.RESULT,
				Status: "ERROR",
				Error:  err.Error(),
			})
			return
		}

        ok, verr := verifyCandidate(ctx, job.Alg, cand, job.FullHash)
        localTested++

        if localTested%flushEvery == 0 {
            flush()
        }

        if verr != nil {
            flush()
            setFinalResult(&messages.ResultMsg{Type: messages.RESULT, Status: "ERROR", Error: verr.Error()})
            return
        }

		if job.CPInterval > 0 && ((g - job.RangeStart + 1) % job.CPInterval) == 0 {
			sendQ <- &messages.CheckpointMsg{
				Type:  messages.CHECK_POINT,
				CpIdx: g,
			}
		}
		
        if ok {
            flush()
			fmt.Printf("[Worker] Password found by thread=%d at index=%d\n", tid, g)
            setFinalResult(&messages.ResultMsg{Type: messages.RESULT, Status: "FOUND", Password: cand})
            return
        }
    }
}

func verifyCandidate(ctx *cryptCtx, alg, candidate, fullHash string) (bool, error) {
	switch alg {
	case "bcrypt":
		err := bcrypt.CompareHashAndPassword([]byte(fullHash), []byte(candidate))
		if err == nil {
			return true, nil
		}
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return false, nil
		}
		return false, err

	case "md5", "sha256", "sha512", "yescrypt":
		got, err := cryptHashWithCtx(ctx, candidate, fullHash) // fullHash is the “setting”
		if err != nil {
			return false, err
		}
		return got == fullHash, nil

	default:
		return false, fmt.Errorf("unsupported alg: %s", strings.TrimSpace(alg))
	}
}
