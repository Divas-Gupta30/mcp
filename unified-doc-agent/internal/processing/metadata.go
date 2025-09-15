package processing

import "time"

type Metadata struct {
	Path       string
	Source     string // "local" or "gdrive"
	ImportedAt time.Time
	Title      string
}
