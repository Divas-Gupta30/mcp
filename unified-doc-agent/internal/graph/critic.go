package graph

import "context"

// Critic can improve or validate the summary. Currently a small heuristic.
func CriticNode(ctx context.Context, s *State) error {
	if len(s.Ans) < 50 {
		s.Ans = s.Ans + "\n\n(Note: result short; consider rephrasing your query or indexing more documents.)"
	}
	return nil
}
