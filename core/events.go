package core

// Event represents any pipeline event
type Event interface {
	EventType() EventType
}

// StatusEvent represents a status change
type StatusEvent struct {
	Status  StatusType
	Target  StatusTarget
	Message string
	Details map[string]any
}

func (e StatusEvent) EventType() EventType {
	return EventTypeStatus
}

// STTEvent represents STT output
type STTEvent struct {
	Text       string
	IsFinal    bool
	Confidence float64
}

func (e STTEvent) EventType() EventType {
	return EventTypeSTT
}

// LLMEvent represents LLM output
type LLMEvent struct {
	Delta   string
	Content string
}

func (e LLMEvent) EventType() EventType {
	return EventTypeLLM
}

// AudioEvent represents TTS audio output
type AudioEvent struct {
	Data   []byte
	Format string
}

func (e AudioEvent) EventType() EventType {
	return EventTypeAudio
}

// ActionEvent represents an action to be executed by the client
type ActionEvent struct {
	ActionID   string
	ActionType ActionType
	Target     string
	Data       map[string]any
	Required   bool
}

func (e ActionEvent) EventType() EventType {
	return EventTypeAction
}

// ErrorEvent represents an error
type ErrorEvent struct {
	Error     error
	Retryable bool
}

func (e ErrorEvent) EventType() EventType {
	return EventTypeError
}

// DoneEvent signals pipeline completion
type DoneEvent struct {
	FullText      string
	TokensUsed    int
	AudioDuration float64
	ActionsCount  int
}

func (e DoneEvent) EventType() EventType {
	return EventTypeDone
}

// ServiceMessageEvent represents a service message for user feedback
type ServiceMessageEvent struct {
	MessageType ServiceMessageType
	Content     string
	Localized   map[string]string // Language code -> localized message
}

func (e ServiceMessageEvent) EventType() EventType {
	return EventTypeServiceMessage
}
