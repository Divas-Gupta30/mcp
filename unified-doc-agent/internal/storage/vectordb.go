package storage

import (
	"context"
	"fmt"

	"github.com/pgvector/pgvector-go"
)

type Document struct {
	ID       int
	Filename string
	Source   string
	Content  string
}

// InsertEmbedding adds a chunk into Postgres with embedding
func InsertEmbedding(filename, source, content string, embedding []float32) error {
	_, err := DB.Exec(context.Background(),
		"INSERT INTO documents (filename, source, content, embedding) VALUES ($1, $2, $3, $4)",
		filename, source, content, pgvector.NewVector(embedding))
	return err
}

// QuerySimilar returns top-k most similar documents
func QuerySimilar(queryEmb []float32, topK int) ([]Document, error) {
	rows, err := DB.Query(context.Background(),
		"SELECT id, filename, source, content FROM documents ORDER BY embedding <-> $1 LIMIT $2",
		pgvector.NewVector(queryEmb), topK)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.ID, &doc.Filename, &doc.Source, &doc.Content); err != nil {
			return nil, err
		}
		results = append(results, doc)
	}
	return results, nil
}

// SearchImpl adapts QuerySimilar for graph.DBWrapper
func SearchImpl(queryEmb []float32, topK int) ([]string, error) {
	docs, err := QuerySimilar(queryEmb, topK)
	if err != nil {
		return nil, err
	}

	results := make([]string, len(docs))
	for i, d := range docs {
		results[i] = fmt.Sprintf("File: %s\n%s", d.Filename, d.Content)
	}
	return results, nil
}
