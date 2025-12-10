package protocol

// InputMessageType defines client-to-server message types
type InputMessageType string

const (
	// User input
	InputText  InputMessageType = "input.text"  // Text message from user
	InputAudio InputMessageType = "input.audio" // Audio chunk from user
	InputEnd   InputMessageType = "input.end"   // End of audio stream

	// Control
	InputCancel InputMessageType = "control.cancel" // Cancel current operation
	InputConfig InputMessageType = "control.config" // Update session config

	// Action response
	InputActionComplete InputMessageType = "action.complete" // Client confirms action completed
)

// InputMessage represents a message from client
type InputMessage struct {
	Type      InputMessageType `json:"type"`
	ID        string           `json:"id"`        // Client-generated message ID
	SessionID string           `json:"sessionId"` // Session identifier
	Payload   any              `json:"payload"`
	Timestamp int64            `json:"timestamp"`
}

// TextInputPayload for input.text
type TextInputPayload struct {
	Text     string         `json:"text"`
	SourceID string         `json:"sourceId,omitempty"` // Source ID for RAG context
	Context  map[string]any `json:"context,omitempty"`  // Additional context
}

// AudioInputPayload for input.audio
type AudioInputPayload struct {
	Data       []byte `json:"data"`       // Base64 encoded audio
	Format     string `json:"format"`     // "pcm", "opus", "wav"
	SampleRate int    `json:"sampleRate"` // e.g., 16000
}

// ConfigPayload for control.config
type ConfigPayload struct {
	Language   string          `json:"language,omitempty"`
	TTSEnabled *bool           `json:"ttsEnabled,omitempty"`
	Providers  ProviderPresets `json:"providers,omitempty"`
}

// ProviderPresets holds preset names for each capability
type ProviderPresets struct {
	LLM       string `json:"llm,omitempty"`
	STT       string `json:"stt,omitempty"`
	TTS       string `json:"tts,omitempty"`
	Embedding string `json:"embedding,omitempty"`
}

// ActionCompletePayload for action.complete (client → server)
type ActionCompletePayload struct {
	ActionID string `json:"actionId"`
	Success  bool   `json:"success"`
	Result   any    `json:"result,omitempty"`
	Error    string `json:"error,omitempty"`
}
