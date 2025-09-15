package ingestion

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/otiai10/gosseract/v2"
)

// ExtractTextWithOCR runs OCR on images or scanned PDFs.
// For PDFs we convert pages to PNGs using pdftoppm (poppler).
func ExtractTextWithOCR(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pdf" {
		// require pdftoppm installed
		tmpPrefix := filepath.Join(os.TempDir(), "uda_pdfimg")
		// pdftoppm -png input.pdf outprefix
		cmd := exec.Command("pdftoppm", "-png", path, tmpPrefix)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("pdftoppm convert failed: %w", err)
		}
		pattern := tmpPrefix + "-*.png"
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", err
		}
		var combined strings.Builder
		for _, m := range matches {
			t, err := runTesseract(m)
			if err != nil {
				continue
			}
			combined.WriteString(t)
			combined.WriteString("\n")
		}
		return strings.TrimSpace(combined.String()), nil
	}
	// image file
	return runTesseract(path)
}

func runTesseract(imgPath string) (string, error) {
	client := gosseract.NewClient()
	defer client.Close()
	if err := client.SetImage(imgPath); err != nil {
		return "", err
	}
	text, err := client.Text()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}
