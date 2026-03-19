package main

import "sync"

/*
ChunkAllocator hands out non-overlapping, sequential index ranges.
	- Start at 1 [0 is nothing]
	- Each chunk is [start, end] both inclusive
	- No chunk range is issued twice
	- Multiple workers can request chunks concurrently
*/
type ChunkAllocator struct {
	mu        sync.Mutex
	nextStart uint64
	chunkSize uint64
	requeue   []Chunk
}

type Chunk struct {
	Start uint64
	End   uint64
}

func NewChunkAllocator(chunkSize uint64) *ChunkAllocator {
	return &ChunkAllocator{nextStart: 1, chunkSize: chunkSize}
}

// Next returns the next chunk range [start,end] (inclusive).
func (a *ChunkAllocator) Next() (start uint64, end uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if len(a.requeue) > 0 {
		requeueChunk := a.requeue[0]
		a.requeue = a.requeue[1:]
		return requeueChunk.Start, requeueChunk.End
	}

	start = a.nextStart
	end = start + a.chunkSize - 1
	a.nextStart = end + 1
	return start, end
}

func (a *ChunkAllocator) Reinsert(start, end uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requeue = append(a.requeue, Chunk{Start: start, End: end})
}