package pipeline_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/creastat/infra/telemetry"
	"github.com/creastat/pipeline/core"
	"github.com/creastat/pipeline/stages"
	providers "github.com/creastat/providers/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock providers (reusing from previous test)
// ... (omitted for brevity, assuming same file or shared mocks)
// Since we are overwriting the file, we need to include mocks again.

// MockSTTProvider
type MockSTTProvider struct{ mock.Mock }

func (m *MockSTTProvider) Name() string                 { return "mock-stt" }
func (m *MockSTTProvider) Type() providers.ProviderType { return providers.ProviderTypeDeepgram }
func (m *MockSTTProvider) Initialize(ctx context.Context, config providers.ProviderConfig) error {
	return nil
}
func (m *MockSTTProvider) Close() error                          { return nil }
func (m *MockSTTProvider) HealthCheck(ctx context.Context) error { return nil }
func (m *MockSTTProvider) Capabilities() []providers.Capability {
	return []providers.Capability{providers.CapabilitySTT}
}
func (m *MockSTTProvider) SupportsCapability(c providers.Capability) bool {
	return c == providers.CapabilitySTT
}
func (m *MockSTTProvider) Transcribe(ctx context.Context, req providers.STTRequest) (*providers.STTResponse, error) {
	return nil, nil
}
func (m *MockSTTProvider) StreamTranscribe(ctx context.Context, req providers.STTRequest) (providers.STTStream, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(providers.STTStream), args.Error(1)
}

type MockSTTStream struct{ mock.Mock }

func (m *MockSTTStream) Send(ctx context.Context, data []byte) error {
	return m.Called(ctx, data).Error(0)
}
func (m *MockSTTStream) Receive(ctx context.Context) (*providers.STTChunk, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providers.STTChunk), args.Error(1)
}
func (m *MockSTTStream) Close() error { return m.Called().Error(0) }

// MockLLMProvider
type MockLLMProvider struct{ mock.Mock }

func (m *MockLLMProvider) Name() string                 { return "mock-llm" }
func (m *MockLLMProvider) Type() providers.ProviderType { return providers.ProviderTypeOpenAI }
func (m *MockLLMProvider) Initialize(ctx context.Context, config providers.ProviderConfig) error {
	return nil
}
func (m *MockLLMProvider) Close() error                          { return nil }
func (m *MockLLMProvider) HealthCheck(ctx context.Context) error { return nil }
func (m *MockLLMProvider) Capabilities() []providers.Capability {
	return []providers.Capability{providers.CapabilityLLM}
}
func (m *MockLLMProvider) SupportsCapability(c providers.Capability) bool {
	return c == providers.CapabilityLLM
}
func (m *MockLLMProvider) ChatCompletion(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return nil, nil
}
func (m *MockLLMProvider) StreamChatCompletion(ctx context.Context, req providers.ChatRequest) (providers.ChatStream, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(providers.ChatStream), args.Error(1)
}

type MockChatStream struct{ mock.Mock }

func (m *MockChatStream) Receive(ctx context.Context) (*providers.ChatChunk, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providers.ChatChunk), args.Error(1)
}
func (m *MockChatStream) Close() error { return m.Called().Error(0) }

func TestSTTToRAGToLLMFlow(t *testing.T) {
	logger := telemetry.New(telemetry.Config{Level: "debug"})

	// Setup mocks
	mockSTT := new(MockSTTProvider)
	mockSTTStream := new(MockSTTStream)
	mockLLM := new(MockLLMProvider)
	mockChatStream := new(MockChatStream)

	// STT Stream behavior
	mockSTTStream.On("Send", mock.Anything, mock.Anything).Return(nil)
	mockSTTStream.On("Receive", mock.Anything).Return(&providers.STTChunk{
		Text: "Hello world", IsFinal: true,
	}, nil).Once()
	mockSTTStream.On("Receive", mock.Anything).Return(nil, io.EOF).Once()
	mockSTTStream.On("Close").Return(nil)
	mockSTT.On("StreamTranscribe", mock.Anything, mock.Anything).Return(mockSTTStream, nil)

	// LLM Stream behavior
	mockChatStream.On("Receive", mock.Anything).Return(&providers.ChatChunk{Content: "Hi"}, nil).Once()
	mockChatStream.On("Receive", mock.Anything).Return(nil, io.EOF).Once()
	mockChatStream.On("Close").Return(nil)

	// Expect LLM to be called with enriched text (RAG fallback)
	mockLLM.On("StreamChatCompletion", mock.Anything, mock.MatchedBy(func(req providers.ChatRequest) bool {
		// RAG fallback is empty string in this test setup, so it just passes query
		return req.Messages[0].Content == "Hello world"
	})).Return(mockChatStream, nil)

	// Create stages
	sttStage := stages.NewSTTStage(stages.STTStageConfig{
		Provider: mockSTT,
		Language: "en",
		Logger:   logger,
	})

	ragStage := stages.NewRAGStage(stages.RAGStageConfig{
		// No vector store, so it uses fallback (empty)
		FallbackContent: "",
		Logger:          logger,
	})

	llmStage := stages.NewLLMStage(stages.LLMStageConfig{
		Provider: mockLLM,
		Model:    "test-model",
		Logger:   logger,
	})

	// Wire them up
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	audioInput := make(chan core.Event, 10)
	sttOutput := make(chan core.Event, 100)
	ragOutput := make(chan core.Event, 100)
	llmOutput := make(chan core.Event, 100)

	// Run stages
	go func() {
		defer close(sttOutput)
		sttStage.Process(ctx, audioInput, sttOutput)
	}()

	go func() {
		defer close(ragOutput)
		ragStage.Process(ctx, sttOutput, ragOutput)
	}()

	go func() {
		defer close(llmOutput)
		llmStage.Process(ctx, ragOutput, llmOutput)
	}()

	// Send audio
	audioInput <- core.AudioEvent{Data: []byte("audio")}
	close(audioInput)

	// Collect results
	var llmEvents []core.LLMEvent
	for event := range llmOutput {
		if e, ok := event.(core.LLMEvent); ok {
			llmEvents = append(llmEvents, e)
		}
	}

	// Verify
	assert.NotEmpty(t, llmEvents)
	assert.Equal(t, "Hi", llmEvents[0].Delta)

	mockSTT.AssertExpectations(t)
	mockSTTStream.AssertExpectations(t)
	mockLLM.AssertExpectations(t)
	mockChatStream.AssertExpectations(t)
}
