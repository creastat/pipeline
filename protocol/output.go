package protocol

// OutputMessageType defines server-to-client message types
type OutputMessageType string

const (
	// Status updates
	OutputStatus OutputMessageType = "status" // Status change notification

	// Streaming content
	OutputStreamSTT   OutputMessageType = "stream.stt"   // STT transcription chunk
	OutputStreamLLM   OutputMessageType = "stream.llm"   // LLM response chunk
	OutputStreamAudio OutputMessageType = "stream.audio" // TTS audio chunk

	// Actions (client-executable commands)
	OutputActionRequest OutputMessageType = "action.request" // Server requests client action

	// Tool execution (server-side tools)
	OutputToolStart  OutputMessageType = "tool.start"  // Tool execution started
	OutputToolResult OutputMessageType = "tool.result" // Tool execution result

	// Lifecycle
	OutputResponseStart      OutputMessageType = "response.start"       // Response generation started
	OutputResponseAudioStart OutputMessageType = "response.audio_start" // Audio stream started
	OutputResponseAudioEnd   OutputMessageType = "response.audio_end"   // Audio stream ended
	OutputResponseEnd        OutputMessageType = "response.end"         // Response complete

	// Service messages
	OutputServiceMessage OutputMessageType = "service.message" // Service message for user feedback

	// Errors
	OutputError OutputMessageType = "error"
)

// OutputMessage represents a message to client
type OutputMessage struct {
	Type      OutputMessageType `json:"type"`
	ID        string            `json:"id"`                // Server-generated message ID
	SessionID string            `json:"sessionId"`         // Session identifier
	ReplyTo   string            `json:"replyTo,omitempty"` // ID of input message
	Payload   any               `json:"payload"`
	Timestamp int64             `json:"timestamp"`
}

// STTStreamPayload for stream.stt
type STTStreamPayload struct {
	Text       string  `json:"text"`
	IsFinal    bool    `json:"isFinal"`
	Confidence float64 `json:"confidence,omitempty"`
}

// LLMStreamPayload for stream.llm
type LLMStreamPayload struct {
	Delta   string `json:"delta"`             // Incremental text
	Content string `json:"content,omitempty"` // Full content so far
}

// AudioStreamPayload for stream.audio
// Note: Audio is typically sent as raw binary WebSocket message
type AudioStreamPayload struct {
	Data   []byte `json:"data,omitempty"` // Audio data (if sent as JSON)
	Format string `json:"format"`         // Audio format
}

// ActionRequestPayload for action.request
type ActionRequestPayload struct {
	ActionID   string         `json:"actionId"`          // Unique action identifier
	ActionType ActionType     `json:"actionType"`        // Type of action
	Target     string         `json:"target,omitempty"`  // Target element/URL
	Data       map[string]any `json:"data,omitempty"`    // Action-specific data
	Required   bool           `json:"required"`          // If true, wait for completion
	Timeout    int            `json:"timeout,omitempty"` // Timeout in ms (0 = no timeout)
}

// ActionType defines the type of action
type ActionType string

const (
	ActionNavigate  ActionType = "navigate"   // Navigate to URL/page
	ActionFillForm  ActionType = "fill_form"  // Fill form fields
	ActionClick     ActionType = "click"      // Click element
	ActionScroll    ActionType = "scroll"     // Scroll to element/position
	ActionShowModal ActionType = "show_modal" // Show modal/dialog
	ActionHideModal ActionType = "hide_modal" // Hide modal/dialog
	ActionNotify    ActionType = "notify"     // Show notification
	ActionDownload  ActionType = "download"   // Trigger download
	ActionCopy      ActionType = "copy"       // Copy to clipboard
	ActionCustom    ActionType = "custom"     // Custom action (app-specific)
)

// ToolStartPayload for tool.start
type ToolStartPayload struct {
	ToolID      string         `json:"toolId"`
	ToolName    string         `json:"toolName"`
	Description string         `json:"description,omitempty"` // Human-readable description
	Input       map[string]any `json:"input"`
}

// ToolResultPayload for tool.result
type ToolResultPayload struct {
	ToolID  string `json:"toolId"`
	Success bool   `json:"success"`
	Output  any    `json:"output"`
	Error   string `json:"error,omitempty"`
}

// ResponseStartPayload for response.start
type ResponseStartPayload struct {
	ResponseID string   `json:"responseId"`
	Sources    []string `json:"sources,omitempty"` // RAG source IDs used
}

// ResponseEndPayload for response.end
type ResponseEndPayload struct {
	ResponseID    string  `json:"responseId"`
	FullText      string  `json:"fullText"` // Complete response text
	TokensUsed    int     `json:"tokensUsed,omitempty"`
	AudioDuration float64 `json:"audioDuration,omitempty"` // TTS duration in seconds
	ActionsCount  int     `json:"actionsCount,omitempty"`  // Number of actions executed
}

// ResponseAudioStartPayload for response.audio_start
type ResponseAudioStartPayload struct {
	ResponseID string `json:"responseId"`
	Encoding   string `json:"encoding"`   // e.g. "pcm", "mp3"
	SampleRate int    `json:"sampleRate"` // e.g. 24000
}

// ResponseAudioEndPayload for response.audio_end
type ResponseAudioEndPayload struct {
	ResponseID string  `json:"responseId"`
	Duration   float64 `json:"duration"` // Duration in seconds
}

// ServiceMessagePayload for service.message
type ServiceMessagePayload struct {
	MessageType string            `json:"messageType"` // retry_request, info, warning
	Content     string            `json:"content"`
	Localized   map[string]string `json:"localized,omitempty"` // Language code -> localized message
}

// ErrorPayload for error messages
type ErrorPayload struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
	Details   any    `json:"details,omitempty"`
}
