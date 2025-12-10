package core

import (
	"testing"

	"pgregory.net/rapid"
)

// For any event type, the EventType() method SHALL return the correct EventType constant.
func TestPropertyEventTypeConsistency(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Test StatusEvent
		statusEvent := StatusEvent{
			Status:  StatusThinking,
			Target:  StatusTargetBot,
			Message: "test",
		}
		if statusEvent.EventType() != EventTypeStatus {
			rt.Fatalf("StatusEvent returned wrong type: %s", statusEvent.EventType())
		}

		// Test STTEvent
		sttEvent := STTEvent{
			Text:       "hello",
			IsFinal:    true,
			Confidence: 0.95,
		}
		if sttEvent.EventType() != EventTypeSTT {
			rt.Fatalf("STTEvent returned wrong type: %s", sttEvent.EventType())
		}

		// Test LLMEvent
		llmEvent := LLMEvent{
			Delta:   "hello",
			Content: "hello world",
		}
		if llmEvent.EventType() != EventTypeLLM {
			rt.Fatalf("LLMEvent returned wrong type: %s", llmEvent.EventType())
		}

		// Test AudioEvent
		audioEvent := AudioEvent{
			Data:   []byte("audio"),
			Format: "pcm",
		}
		if audioEvent.EventType() != EventTypeAudio {
			rt.Fatalf("AudioEvent returned wrong type: %s", audioEvent.EventType())
		}

		// Test ActionEvent
		actionEvent := ActionEvent{
			ActionID:   "action_1",
			ActionType: ActionNavigate,
			Target:     "/test",
			Required:   true,
		}
		if actionEvent.EventType() != EventTypeAction {
			rt.Fatalf("ActionEvent returned wrong type: %s", actionEvent.EventType())
		}

		// Test ErrorEvent
		errorEvent := ErrorEvent{
			Error:     nil,
			Retryable: false,
		}
		if errorEvent.EventType() != EventTypeError {
			rt.Fatalf("ErrorEvent returned wrong type: %s", errorEvent.EventType())
		}

		// Test DoneEvent
		doneEvent := DoneEvent{
			FullText:      "response",
			TokensUsed:    100,
			AudioDuration: 1.5,
			ActionsCount:  0,
		}
		if doneEvent.EventType() != EventTypeDone {
			rt.Fatalf("DoneEvent returned wrong type: %s", doneEvent.EventType())
		}
	})
}

// For any event type constant, it SHALL have a non-empty string value.
func TestPropertyEventTypeConstants(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		eventTypes := []EventType{
			EventTypeStatus,
			EventTypeSTT,
			EventTypeLLM,
			EventTypeAudio,
			EventTypeAction,
			EventTypeError,
			EventTypeDone,
		}

		for _, et := range eventTypes {
			if et == "" {
				rt.Fatalf("Event type is empty")
			}
		}
	})
}

// For any status type constant, it SHALL have a non-empty string value.
func TestPropertyStatusTypeConstants(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		statusTypes := []StatusType{
			StatusListening,
			StatusTranscribing,
			StatusSearching,
			StatusThinking,
			StatusSpeaking,
			StatusExecuting,
			StatusIdle,
		}

		for _, st := range statusTypes {
			if st == "" {
				rt.Fatalf("Status type is empty")
			}
		}
	})
}

// For any action type constant, it SHALL have a non-empty string value.
func TestPropertyActionTypeConstants(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		actionTypes := []ActionType{
			ActionNavigate,
			ActionFillForm,
			ActionClick,
			ActionScroll,
			ActionShowModal,
			ActionHideModal,
			ActionNotify,
			ActionDownload,
			ActionCopy,
			ActionCustom,
		}

		for _, at := range actionTypes {
			if at == "" {
				rt.Fatalf("Action type is empty")
			}
		}
	})
}
