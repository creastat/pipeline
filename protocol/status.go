package protocol

// StatusType defines the current processing status
type StatusType string

const (
	StatusListening    StatusType = "listening"    // Waiting for input
	StatusTranscribing StatusType = "transcribing" // Processing audio (STT) - shown in user bubble
	StatusSearching    StatusType = "searching"    // RAG: Searching knowledge base
	StatusThinking     StatusType = "thinking"     // LLM: Generating response
	StatusSpeaking     StatusType = "speaking"     // TTS: Generating audio
	StatusExecuting    StatusType = "executing"    // Executing action
	StatusIdle         StatusType = "idle"         // No active operation
)

// StatusTarget defines where the status should be displayed
type StatusTarget string

const (
	StatusTargetUser StatusTarget = "user" // Show in user message bubble (e.g., transcribing)
	StatusTargetBot  StatusTarget = "bot"  // Show in bot message bubble (e.g., thinking)
)

// StatusPayload for status messages
type StatusPayload struct {
	Status  StatusType   `json:"status"`
	Target  StatusTarget `json:"target"`            // Where to display: "user" or "bot"
	Message string       `json:"message,omitempty"` // Human-readable description
	Details any          `json:"details,omitempty"` // Additional details
}

// Status display mapping:
// - "transcribing" → user bubble (shows user is speaking)
// - "searching"    → bot bubble (shows bot is working)
// - "thinking"     → bot bubble (shows bot is working)
// - "speaking"     → bot bubble (shows bot is responding)
// - "executing"    → bot bubble (shows bot is taking action)
