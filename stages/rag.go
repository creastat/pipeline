package stages

import (
	"context"
	"fmt"
	"strings"

	"github.com/creastat/infra/telemetry"
	"github.com/creastat/pipeline/core"
	providers "github.com/creastat/providers/core"
	"github.com/creastat/storage/vectorstore"
)

// DocumentMetadata represents minimal document information for enriching RAG context.
type DocumentMetadata struct {
	Title string
	URL   string
}

// DocumentMetadataProvider is an optional interface for enriching RAG results with document metadata.
// Implementations fetch document details (title, URL, etc.) to add context to retrieved chunks.
type DocumentMetadataProvider interface {
	GetDocumentMetadata(ctx context.Context, documentID string) (*DocumentMetadata, error)
}

// RAGStageConfig holds RAG stage configuration.
type RAGStageConfig struct {
	// VectorStore is the vector store to search.
	VectorStore vectorstore.VectorStore

	// EmbeddingProvider generates embeddings for queries.
	EmbeddingProvider providers.EmbeddingProvider

	// EmbeddingModel is the model to use for embeddings.
	EmbeddingModel string

	// SourceID filters results to a specific source.
	// Deprecated: Use SourceIDs for filtering by multiple sources.
	SourceID string

	// SourceIDs filters results to multiple sources.
	// Takes precedence over SourceID if set.
	SourceIDs []string

	// Threshold is the minimum similarity score (0.0-1.0).
	Threshold float32

	// MaxChunks is the maximum number of chunks to retrieve.
	MaxChunks int

	// FallbackContent is used when RAG fails or returns no results.
	FallbackContent string

	// MetadataProvider is an optional provider for enriching results with document metadata.
	// If provided, RAG stage will fetch document titles and URLs to add to the context.
	MetadataProvider DocumentMetadataProvider

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

	// Collect query text from input
	var queryText string
	for event := range input {
		if llmEvent, ok := event.(core.LLMEvent); ok {
			queryText += llmEvent.Delta
			logger.Debug("Received LLM event", telemetry.String("delta", llmEvent.Delta))
		} else if _, ok := event.(core.DoneEvent); ok {
			// Stop collecting on DoneEvent
			logger.Info("Received DoneEvent, finishing collection")
			break
		}
	}

	if queryText == "" {
		logger.Info("No query text received, finishing stage silently")
		// Emit DoneEvent to signal completion
		output <- core.DoneEvent{}
		return nil
	}

	// Emit searching status only when we actually have a query to search for
	output <- core.StatusEvent{
		Status:  core.StatusSearching,
		Target:  core.StatusTargetBot,
		Message: "Searching knowledge base...",
	}

	logger.Info("Collected query text", telemetry.String("query", queryText))

	// Build context
	ragContext, err := s.buildContext(ctx, queryText)
	if err != nil {
		// Log error but continue with fallback silently (don't send to client)
		logger.Error("RAG context building failed, using fallback", telemetry.Err(err))
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
	logger.Info("Emitting DoneEvent")
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

	// Build search filter
	filter := vectorstore.SearchFilter{
		MinScore: s.config.Threshold,
	}

	// Use SourceIDs if provided, otherwise fall back to SourceID for backward compatibility
	if len(s.config.SourceIDs) > 0 {
		filter.SourceIDs = s.config.SourceIDs
	} else if s.config.SourceID != "" {
		filter.SourceID = s.config.SourceID
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

		contextEntry := result.Content

		// Enrich with document metadata if provider is available
		if s.config.MetadataProvider != nil && result.DocumentID != "" {
			if doc, err := s.config.MetadataProvider.GetDocumentMetadata(ctx, result.DocumentID); err == nil && doc != nil {
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
