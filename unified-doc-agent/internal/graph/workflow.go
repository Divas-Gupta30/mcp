package graph

import "context"

type State struct {
	Query string
	Docs  []string
	Ans   string
	DB    *DBWrapper
}

// DBWrapper is a thin wrapper around VectorStore to avoid circular imports in this example.
// In real code, just pass the vector store interface type.
type DBWrapper struct {
	Search func([]float32, int) ([]string, error)
}

func RunWorkflow(ctx context.Context, s *State) error {
	nodes := []func(context.Context, *State) error{
		RetrieverNode,
		SummarizerNode,
		CriticNode,
		AnswerNode,
	}
	for _, n := range nodes {
		if err := n(ctx, s); err != nil {
			return err
		}
	}
	return nil
}
