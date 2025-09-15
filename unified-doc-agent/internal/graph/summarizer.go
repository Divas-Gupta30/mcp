package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// request body for Ollama
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// Ollama streaming response chunks look like { "response": "...", "done": false }
// We only care about "response".
type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func SummarizerNode(ctx context.Context, s *State) error {
	if len(s.Docs) == 0 {
		s.Ans = "No documents found matching the query."
		return nil
	}

	// Build the summarization prompt
	var docText strings.Builder
	for i, d := range s.Docs {
		docText.WriteString(fmt.Sprintf("Document %d:\n%s\n\n", i+1, d))
	}

	prompt := fmt.Sprintf(
		"The user asked: %q.\n\nSummarize the following documents in the context of this query:\n\n%s",
		s.Query,
		docText.String(),
	)
	// Prepare request
	reqBody, _ := json.Marshal(ollamaRequest{
		Model:  "llama3", // change if you want another model like "mistral"
		Prompt: prompt,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:11434/api/generate", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("creating ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Call Ollama
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling ollama: %w", err)
	}
	defer resp.Body.Close()

	// Read streaming response
	var summary strings.Builder
	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk ollamaResponse
		if err := decoder.Decode(&chunk); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("decoding ollama response: %w", err)
		}
		summary.WriteString(chunk.Response)
		if chunk.Done {
			break
		}
	}

	s.Ans = summary.String()
	return nil
}
