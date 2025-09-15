package ingestion

import (
	"io/fs"
	"path/filepath"
	"strings"
)

var allowedExt = []string{".pdf", ".txt", ".md", ".png", ".jpg", ".jpeg"}

func LoadLocalFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		for _, a := range allowedExt {
			if ext == a {
				out = append(out, path)
				break
			}
		}
		return nil
	})
	return out, err
}
