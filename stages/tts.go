package stages

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/creastat/infra/telemetry"
	"github.com/creastat/pipeline/core"
	providers "github.com/creastat/providers/core"
)

// TTSStageConfig holds TTS stage configuration
type TTSStageConfig struct {
	Provider providers.TTSProvider
	Voice    string
	Language string
	Speed    *float64
	Encoding string
	Logger   telemetry.Logger
}

// TTSStage represents a text-to-speech processing stage
type TTSStage struct {
	config TTSStageConfig
}

// NewTTSStage creates a new TTS stage
func NewTTSStage(config TTSStageConfig) *TTSStage {
	return &TTSStage{
		config: config,
	}
}

// Name returns the stage name
func (s *TTSStage) Name() string {
	return "tts"
}

// logError logs TTS errors without sending them to the user
func (s *TTSStage) logError(err error) {
	// Errors are logged at the service level, not sent to client
	// This prevents TTS-specific errors from disrupting the user experience
	_ = err // Placeholder for logging integration
}

// InputTypes returns the event types this stage accepts
func (s *TTSStage) InputTypes() []core.EventType {
	return []core.EventType{core.EventTypeLLM}
}

// OutputTypes returns the event types this stage produces
func (s *TTSStage) OutputTypes() []core.EventType {
	return []core.EventType{core.EventTypeAudio, core.EventTypeStatus, core.EventTypeDone}
}

// Process implements the Stage interface
// Note: Text buffering and cleaning is handled by TextProcessorStage upstream.
// This stage receives pre-processed, sentence-complete text and focuses solely on TTS synthesis.
func (s *TTSStage) Process(ctx context.Context, input <-chan core.Event, output chan<- core.Event) error {
	logger := s.config.Logger.WithModule(s.Name())

	logger.Info("Emitting speaking status")

	// Emit speaking status
	output <- core.StatusEvent{
		Status:  core.StatusSpeaking,
		Target:  core.StatusTargetBot,
		Message: "Speaking...",
	}

	// Channels for coordination
	textChan := make(chan string, 100)
	audioChan := make(chan core.Event, 100)
	errChan := make(chan error, 2)

	var wg sync.WaitGroup
	var stream providers.TTSStream
	var streamErr error
	var streamOnce sync.Once
	streamReady := make(chan struct{})

	// Helper to initialize stream safely
	initStream := func() bool {
		streamOnce.Do(func() {
			logger.Info("Starting TTS stream", telemetry.String("provider", s.config.Provider.Name()), telemetry.String("language", s.config.Language), telemetry.String("voice", s.config.Voice))
			stream, streamErr = s.config.Provider.StreamSynthesize(ctx, providers.TTSRequest{
				Voice:    s.config.Voice,
				Language: s.config.Language,
				Speed:    s.config.Speed,
			})
			if streamErr != nil {
				logger.Error("Failed to start TTS stream", telemetry.Err(streamErr), telemetry.String("provider", s.config.Provider.Name()), telemetry.String("language", s.config.Language))
				output <- core.ErrorEvent{
					Error:     fmt.Errorf("failed to start TTS stream (provider=%s, language=%s): %w", s.config.Provider.Name(), s.config.Language, streamErr),
					Retryable: true,
				}
				// Signal ready even on error so waiters can unblock and see the error
				close(streamReady)
				return
			}
			close(streamReady)
		})
		return streamErr == nil
	}

	// Goroutine: Send text to TTS stream
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Wait for stream to be initialized
		select {
		case <-streamReady:
		case <-ctx.Done():
			return
		}

		if stream == nil {
			return
		}

		defer func() {
			// After all text is sent, call Finish() if supported
			// This must be done BEFORE waiting for audio to avoid deadlock
			logger.Trace("Text sending complete, calling Finish() if supported")
			if finisher, ok := stream.(interface{ Finish(context.Context) error }); ok {
				logger.Trace("Stream supports Finish(), calling it now", telemetry.String("provider", s.config.Provider.Name()))
				if err := finisher.Finish(ctx); err != nil {
					logger.Error("Failed to finish TTS stream", telemetry.Err(err))
				} else {
					logger.Info("Successfully called Finish() on TTS provider", telemetry.String("provider", s.config.Provider.Name()))
				}
			} else {
				logger.Trace("Stream does not support Finish() (not Minimax)", telemetry.String("provider", s.config.Provider.Name()))
			}
		}()

		for text := range textChan {
			if err := stream.Send(ctx, text); err != nil {
				logger.Error("Failed to send text to TTS stream", telemetry.Err(err))
				select {
				case errChan <- fmt.Errorf("failed to send text to TTS: %w", err):
				default:
				}
				return
			}
			logger.Trace("Sent text to TTS provider", telemetry.String("text", text), telemetry.String("provider", s.config.Provider.Name()))
		}
		logger.Trace("Text channel closed, text-sending goroutine exiting")
	}()

	// Goroutine: Receive audio from TTS stream
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(audioChan)

		// Wait for stream to be initialized
		select {
		case <-streamReady:
		case <-ctx.Done():
			return
		}

		if stream == nil {
			return
		}
		defer stream.Close()

		var audioChunkCount int
		var firstChunkLogged bool

		for {
			chunk, err := stream.Receive(ctx)
			if err != nil {
				// If the error is EOF or similar "done" error, treat it as success
				if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "stream closed") {
					logger.Info("TTS stream finished (EOF)", telemetry.Int("chunks_received", audioChunkCount))
					return
				}

				// Log other errors but check if we received any chunks
				// Some providers might return an error at the end of a successful stream
				if audioChunkCount > 0 {
					logger.Warn("Error receiving TTS chunk after successful stream", telemetry.Err(err), telemetry.Int("chunks_received", audioChunkCount))
					// Don't emit DoneEvent here - let the main loop handle it
					return
				}

				logger.Error("Error receiving TTS chunk", telemetry.Err(err), telemetry.Int("chunks_received", audioChunkCount))
				select {
				case errChan <- fmt.Errorf("error receiving TTS chunk: %w", err):
				default:
				}
				return
			}

			if chunk == nil || chunk.Done {
				logger.Info("TTS stream finished", telemetry.Int("chunks_received", audioChunkCount))
				return
			}

			audioChunkCount++
			if !firstChunkLogged {
				logger.Debug("Received audio chunk and forwarding audio event", telemetry.Int("size", len(chunk.Audio)))
				firstChunkLogged = true
			} else {
				logger.Debug("Received audio chunk and forwarding audio event", telemetry.Int("size", len(chunk.Audio)), telemetry.Int("chunk_number", audioChunkCount))
			}

			select {
			case <-ctx.Done():
				return
			case audioChan <- core.AudioEvent{
				Data:   chunk.Audio,
				Format: s.config.Encoding,
			}:
			}
		}
	}()

	// Goroutine: Process input events (already cleaned and buffered by TextProcessorStage)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(textChan)

		for event := range input {
			if statusEvent, ok := event.(core.StatusEvent); ok {
				logger.Debug("Forwarding status event", telemetry.String("status", string(statusEvent.Status)))
				output <- statusEvent
				continue
			}

			if llmEvent, ok := event.(core.LLMEvent); ok {
				if strings.TrimSpace(llmEvent.Delta) == "" {
					continue
				}

				// Initialize stream on first text chunk
				if !initStream() {
					return
				}

				logger.Trace("Received text for TTS", telemetry.String("text", llmEvent.Delta))

				select {
				case <-ctx.Done():
					return
				case textChan <- llmEvent.Delta:
				}
			}

			// If we receive a DoneEvent, signal end of text and stop processing
			if _, ok := event.(core.DoneEvent); ok {
				logger.Info("Received DoneEvent, signaling end of text to TTS provider")

				// Wait for text-sending goroutine to finish
				// This ensures all text is sent before we call Finish()
				logger.Trace("Waiting for text-sending goroutine to complete")

				// Don't call Finish() here - do it after wg.Wait() in main loop
				// to avoid concurrent writes to WebSocket

				// Do NOT forward DoneEvent here.
				// Forwarding it causes downstream stages (ws_sink) to think audio is done
				// while we are still streaming audio.
				// We will emit our own DoneEvent when audio streaming is actually complete.
				return
			}
		}
	}()

	// Main loop: Forward audio chunks and handle errors
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-errChan:
			if err != nil {
				// Log TTS errors but don't send to user - TTS errors are internal
				logger.Error("TTS error", telemetry.Err(err))
				s.logError(err)
				return err
			}

		case event, ok := <-audioChan:
			if !ok {
				// Audio channel closed, wait for all goroutines
				wg.Wait()

				// Check for any errors
				select {
				case err := <-errChan:
					if err != nil {
						// Log TTS errors but don't send to user
						logger.Error("TTS error", telemetry.Err(err))
						s.logError(err)
						return err
					}
				default:
				}

				// Emit done event (no service message for empty content - it's handled upstream)
				logger.Info("Emitting done event")
				output <- core.DoneEvent{
					AudioDuration: 0,
				}
				return nil
			}

			if audioEvent, ok := event.(core.AudioEvent); ok {
				output <- audioEvent
			}
		}
	}
}
