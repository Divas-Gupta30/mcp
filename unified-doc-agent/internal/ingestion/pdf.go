package ingestion

import (
	"bytes"
	"io"
	"os/exec"
	"strings"

	pdf "github.com/ledongthuc/pdf"
)

// ExtractTextFromPDF tries to extract text; returns empty string if none found.
func ExtractTextFromPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		// fallback to pdftotext later
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	_, err = io.Copy(&buf, b)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(buf.String())
	if text == "" {
		// try pdftotext CLI if available
		out, err := exec.Command("pdftotext", "-layout", path, "-").Output()
		if err == nil {
			return string(out), nil
		}
	}
	return text, nil
}
