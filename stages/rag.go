package stages

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/creastat/infra/telemetry"
	"github.com/creastat/pipeline/core"
	providers "github.com/creastat/providers/core"
	"github.com/creastat/storage/vectorstore"
)

// RAGStageConfig holds RAG stage configuration.
type RAGStageConfig struct {
	// VectorStore is the vector store to search.
	VectorStore vectorstore.VectorStore

	// EmbeddingProvider generates embeddings for queries.
	EmbeddingProvider providers.EmbeddingProvider

	// EmbeddingModel is the model to use for embeddings.
	EmbeddingModel string

	// SourceID filters results to a specific source.
	SourceID string

	// Threshold is the minimum similarity score (0.0-1.0).
	Threshold float32

	// MaxChunks is the maximum number of chunks to retrieve.
	MaxChunks int

	// FallbackContent is used when RAG fails or returns no results.
	FallbackContent string

	// SupabaseStore is optional and used to enrich results with document metadata.
	SupabaseStore any // supabase.Store

	Logger telemetry.Logger
}

// RAGStage retrieves relevant context from a vector store.
type RAGStage struct {
	config RAGStageConfig
}

// NewRAGStage creates a new RAG stage.
func NewRAGStage(config RAGStageConfig) *RAGStage {
	if config.MaxChunks <= 0 {
		config.MaxChunks = 5
	}
	if config.Threshold <= 0 {
		config.Threshold = 0.7
	}
	return &RAGStage{config: config}
}

// Name returns the stage name.
func (s *RAGStage) Name() string {
	return "rag"
}

// InputTypes returns the event types this stage accepts
func (s *RAGStage) InputTypes() []core.EventType {
	return []core.EventType{core.EventTypeLLM}
}

// OutputTypes returns the event types this stage produces
func (s *RAGStage) OutputTypes() []core.EventType {
	return []core.EventType{core.EventTypeLLM, core.EventTypeStatus}
}

// Process implements the Stage interface.
// It reads the query from input, retrieves context, and passes enriched input to output.
func (s *RAGStage) Process(ctx context.Context, input <-chan core.Event, output chan<- core.Event) error {
	logger := s.config.Logger.WithModule(s.Name())
	logger.Info("RAGStage started processing")

	// Emit searching status
	output <- core.StatusEvent{
		Status:  core.StatusSearching,
		Target:  core.StatusTargetBot,
		Message: "Searching knowledge base...",
	}

	// Collect query text from input
	var queryText string
	for event := range input {
		if llmEvent, ok := event.(core.LLMEvent); ok {
			queryText += llmEvent.Delta
			logger.Debug("received LLM event", telemetry.String("delta", llmEvent.Delta))
		} else if _, ok := event.(core.DoneEvent); ok {
			// Stop collecting on DoneEvent
			logger.Info("received DoneEvent, finishing collection")
			break
		} else {
			// Forward other events (like ServiceMessageEvent, StatusEvent, etc.) to output
			logger.Debug("forwarding non-LLM event", telemetry.String("event_type", string(event.EventType())))
			output <- event
		}
	}

	if queryText == "" {
		logger.Warn("no query text received")
		// Don't propagate error - STT already sent a retry message
		// Just emit DoneEvent to signal completion
		logger.Info("emitting DoneEvent with no query text")
		output <- core.DoneEvent{}
		return nil
	}

	logger.Info("collected query text", telemetry.String("query", queryText))

	// Build context
	ragContext, err := s.buildContext(ctx, queryText)
	if err != nil {
		// Log error but continue with fallback
		logger.Error("RAG context building failed, using fallback", telemetry.Err(err))
		output <- core.ErrorEvent{
			Error:     fmt.Errorf("RAG context building failed, using fallback: %w", err),
			Retryable: false,
		}
		ragContext = s.config.FallbackContent
	}

	// If no context found, use fallback
	if ragContext == "" {
		logger.Info("no context found, using fallback")
		ragContext = s.config.FallbackContent
	} else {
		logger.Info("found context", telemetry.Int("context_length", len(ragContext)))
	}

	// Pass the original query with context to the next stage
	// The context will be prepended to the query
	enrichedQuery := queryText
	if ragContext != "" {
		enrichedQuery = fmt.Sprintf("Context:\n%s\n\nQuestion: %s", ragContext, queryText)
	}

	output <- core.LLMEvent{
		Delta:   enrichedQuery,
		Content: enrichedQuery,
	}

	// Emit DoneEvent to signal completion to downstream stages (like LLM)
	logger.Info("emitting DoneEvent")
	output <- core.DoneEvent{
		FullText: enrichedQuery,
	}

	return nil
}

// buildContext generates embedding and searches vector store.
func (s *RAGStage) buildContext(ctx context.Context, query string) (string, error) {
	// Skip if no vector store or embedding provider
	if s.config.VectorStore == nil || s.config.EmbeddingProvider == nil {
		return s.config.FallbackContent, nil
	}

	// Generate embedding for query
	embResp, err := s.config.EmbeddingProvider.GenerateEmbedding(ctx, providers.EmbeddingRequest{
		Model: s.config.EmbeddingModel,
		Text:  query,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Search vector store
	filter := vectorstore.SearchFilter{
		SourceID: s.config.SourceID,
		MinScore: s.config.Threshold,
	}

	results, err := s.config.VectorStore.Search(ctx, embResp.Vector, filter, s.config.MaxChunks)
	if err != nil {
		return "", fmt.Errorf("vector search failed: %w", err)
	}

	if len(results) == 0 {
		return "", nil
	}

	// Format context from results
	var contextParts []string
	for _, result := range results {
		if result.Content == "" {
			continue
		}

		// Build context entry with optional document metadata
		contextEntry := result.Content

		// If Supabase store is available, enrich with document metadata
		if s.config.SupabaseStore != nil && result.DocumentID != "" {
			// Try to get document details
			// Note: We use reflection to avoid circular dependency
			// In production, this should be properly typed
			if doc, err := s.getDocumentMetadata(ctx, result.DocumentID); err == nil && doc != nil {
				// Prepend document title and URL if available
				if doc.Title != "" {
					contextEntry = fmt.Sprintf("**%s**\n%s", doc.Title, contextEntry)
				}
				if doc.URL != "" {
					contextEntry = fmt.Sprintf("%s\n(Source: %s)", contextEntry, doc.URL)
				}
			}
		}

		contextParts = append(contextParts, contextEntry)
	}

	return strings.Join(contextParts, "\n\n---\n\n"), nil
}

// DocumentMetadata represents minimal document info for enrichment
type DocumentMetadata struct {
	Title string
	URL   string
}

// getDocumentMetadata retrieves document metadata from Supabase store
func (s *RAGStage) getDocumentMetadata(ctx context.Context, documentID string) (*DocumentMetadata, error) {
	if s.config.SupabaseStore == nil {
		return nil, fmt.Errorf("supabase store not configured")
	}

	// Use reflection to call GetDocument on the store
	// This avoids circular dependency on the supabase package
	storeValue := reflect.ValueOf(s.config.SupabaseStore)
	method := storeValue.MethodByName("GetDocument")
	if !method.IsValid() {
		return nil, fmt.Errorf("GetDocument method not found")
	}

	// Call GetDocument(ctx, documentID)
	results := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(documentID),
	})

	if len(results) < 2 {
		return nil, fmt.Errorf("unexpected return values")
	}

	// Check for error
	if !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}

	// Extract document
	if results[0].IsNil() {
		return nil, nil
	}

	doc := results[0].Interface()
	docValue := reflect.ValueOf(doc)

	// Extract Title and URL fields
	title := ""
	url := ""

	if titleField := docValue.FieldByName("Title"); titleField.IsValid() {
		title = titleField.String()
	}
	if urlField := docValue.FieldByName("URL"); urlField.IsValid() {
		url = urlField.String()
	}

	return &DocumentMetadata{
		Title: title,
		URL:   url,
	}, nil
}
