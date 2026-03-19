package main

type ChunkAssignMsg struct {
	Type  string `json:"type"` // "CHUNK_ASSIGN" (or "CHUNK")
	Start uint64 `json:"start"`
	End   uint64 `json:"end"` // End is exclusive: [Start, End)
	ChunkID uint64 `json:"chunk_id,omitempty"`
}

type ChunkDoneMsg struct {
	Type    string `json:"type"` // "CHUNK_DONE"
	Status  string `json:"status"` // "NOT_FOUND" or "ERROR"
	Start   uint64 `json:"start,omitempty"`
	End     uint64 `json:"end,omitempty"`
	ChunkID uint64 `json:"chunk_id,omitempty"`
	Error   string `json:"error,omitempty"`
}
