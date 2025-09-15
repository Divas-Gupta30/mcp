package graph

import (
	"context"
	"fmt"
)

// AnswerNode prints the final answer to console. You can replace it to return JSON.
func AnswerNode(ctx context.Context, s *State) error {
	fmt.Println("\n===== ANSWER =====\n")
	fmt.Println(s.Ans)
	return nil
}
