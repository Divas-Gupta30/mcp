package processing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

// EmbeddingDim is the fixed dimension of the embedding vector.
const EmbeddingDim = 768

// Ollama API endpoint for local embeddings
const ollamaURL = "http://localhost:11434/api/embeddings"

// request struct for Ollama API
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// response struct from Ollama API
type ollamaResponse struct {
	Embedding []float32 `json:"embedding"`
}

// EmbedChunks produces embeddings for each chunk by calling Ollama.
func EmbedChunks(ctx context.Context, chunks []string) ([][]float32, error) {
	if len(chunks) == 0 {
		return nil, errors.New("no chunks")
	}

	out := make([][]float32, len(chunks))
	for i, chunk := range chunks {
		emb, err := getOllamaEmbedding(chunk)
		if err != nil {
			return nil, fmt.Errorf("failed embedding chunk %d: %w", i, err)
		}
		out[i] = emb
	}

	return out, nil
}

// QueryEmbedding produces an embedding for a query string.
func QueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	if query == "" {
		return nil, errors.New("empty query")
	}
	return getOllamaEmbedding(query)
}

// getOllamaEmbedding calls Ollama local API and returns the embedding vector.
func getOllamaEmbedding(text string) ([]float32, error) {
	reqBody := ollamaRequest{
		Model:  "nomic-embed-text",
		Prompt: text,
	}
	data, _ := json.Marshal(reqBody)

	resp, err := http.Post(ollamaURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error: %s", string(bodyBytes))
	}

	var oResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return nil, fmt.Errorf("failed decode response: %w", err)
	}

	if len(oResp.Embedding) != EmbeddingDim {
		return nil, fmt.Errorf("expected embedding dim %d, got %d", EmbeddingDim, len(oResp.Embedding))
	}

	return oResp.Embedding, nil
}
