package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Divas-Gupta30/mcp/unified-doc-agent/internal/graph"
	"github.com/Divas-Gupta30/mcp/unified-doc-agent/internal/ingestion"
	"github.com/Divas-Gupta30/mcp/unified-doc-agent/internal/processing"
	"github.com/Divas-Gupta30/mcp/unified-doc-agent/internal/storage"
)

func main() {

	indexCmd := flag.NewFlagSet("index", flag.ExitOnError)
	indexPath := indexCmd.String("path", "./data", "path to folder to index")

	queryCmd := flag.NewFlagSet("query", flag.ExitOnError)
	queryText := queryCmd.String("q", "", "query text")

	if len(os.Args) < 2 {
		fmt.Println("Usage: agent <index|query> [flags]")
		os.Exit(1)
	}

	// Init Postgres DB
	if err := storage.InitDB(); err != nil {
		log.Fatal("DB init:", err)
	}

	switch os.Args[1] {
	case "index":
		indexCmd.Parse(os.Args[2:])
		log.Println("Starting indexing:", *indexPath)

		files, err := ingestion.LoadLocalFiles(*indexPath)
		if err != nil {
			log.Fatal("load files:", err)
		}

		for _, f := range files {
			log.Println("Indexing:", f)
			text, err := ingestion.ExtractText(f)
			if err != nil {
				log.Println("skip file:", f, "err:", err)
				continue
			}
			chunks := processing.ChunkText(text)
			embs, err := processing.EmbedChunks(context.Background(), chunks)
			if err != nil {
				log.Println("embed error:", err)
				continue
			}
			for i := range chunks {
				if err := storage.InsertEmbedding(f, "local", chunks[i], embs[i]); err != nil {
					log.Println("db insert error:", err)
				}
			}
		}
		fmt.Println("Indexing complete.")

	case "query":
		queryCmd.Parse(os.Args[2:])
		if *queryText == "" {
			fmt.Println("Please provide -q \"your query\"")
			os.Exit(1)
		}

		state := &graph.State{
			Query: *queryText,
			Docs:  nil, // RetrieverNode will fill this
			Ans:   "",
			DB: &graph.DBWrapper{
				Search: storage.SearchImpl, // inject search implementation
			},
		}

		if err := graph.RunWorkflow(context.Background(), state); err != nil {
			log.Fatal(err)
		}

		fmt.Println("Answer:", state.Ans)

	default:
		fmt.Println("expected 'index' or 'query' subcommands")
		os.Exit(1)
	}
}

// helper to convert storage.Document → string slice for graph.State
func convertDocs(docs []storage.Document) []string {
	out := make([]string, len(docs))
	for i, d := range docs {
		out[i] = fmt.Sprintf("File: %s\n%s", d.Filename, d.Content)
	}
	return out
}
