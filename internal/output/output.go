package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/1ncrease0/crawler/internal/model"
)

func Write(path string, roots []*model.Node) error {
	if roots == nil {
		roots = []*model.Node{}
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create output file %q: %w", path, err)
	}

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(roots); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode output: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close output file %q: %w", path, err)
	}
	return nil
}
