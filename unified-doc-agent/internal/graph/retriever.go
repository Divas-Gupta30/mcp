package graph

import (
	"context"

	"github.com/Divas-Gupta30/mcp/unified-doc-agent/internal/processing"
)

func RetrieverNode(ctx context.Context, s *State) error {
	qemb, err := processing.QueryEmbedding(ctx, s.Query)
	if err != nil {
		return err
	}
	docs, err := s.DB.Search(qemb, 5)
	if err != nil {
		return err
	}
	s.Docs = docs
	return nil
}
