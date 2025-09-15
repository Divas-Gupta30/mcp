package ingestion

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ExtractText detects file type and returns text via direct extraction or OCR.
func ExtractText(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md":
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case ".pdf":
		// try text layer
		text, err := ExtractTextFromPDF(path)
		if err == nil && strings.TrimSpace(text) != "" {
			return text, nil
		}
		//fallback to OCR
		return ExtractTextWithOCR(path)
	case ".png", ".jpg", ".jpeg":
		return ExtractTextWithOCR(path)
	default:
		return "", errors.New("unsupported file type")
	}
}
