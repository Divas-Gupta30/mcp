unified-doc-agent/
├── cmd/
│   └── agent/              # Main CLI entrypoint
│       └── main.go
├── internal/
│   ├── ingestion/          # File loading + OCR + parsing
│   │   ├── local.go
│   │   ├── gdrive.go
│   │   ├── pdf.go
│   │   └── ocr.go
│   ├── processing/         # Chunking, embeddings, metadata
│   │   ├── chunker.go
│   │   ├── embeddings.go
│   │   └── metadata.go
│   ├── storage/            # Vector DB + metadata DB
│   │   ├── vectordb.go
│   │   └── sqlitedb.go
│   ├── graph/              # LangGraph-like orchestration
│   │   ├── retriever.go
│   │   ├── summarizer.go
│   │   ├── critic.go
│   │   └── workflow.go
│   ├── query/              # User query interface
│     ├── search.go
│     └── answer.go
│   
├
├── go.mod
├── go.sum
└── README.md
