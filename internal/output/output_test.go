package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/1ncrease0/crawler/internal/model"
)

func TestWrite(t *testing.T) {
	t.Parallel()

	roots := []*model.Node{
		{
			Resource: "https://example.com",
			Title:    "Example",
			Links: []*model.Node{
				{Resource: "https://example.com/a", Title: "A", Links: []*model.Node{}},
			},
		},
	}
	path := filepath.Join(t.TempDir(), "result.json")

	require.NoError(t, Write(path, roots))

	var got []*model.Node
	require.NoError(t, json.Unmarshal(readFile(t, path), &got))
	assert.Equal(t, roots, got)
}

func TestWriteEmpty(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "result.json")

	require.NoError(t, Write(path, nil))

	assert.JSONEq(t, "[]", string(readFile(t, path)))
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing", "result.json")

	assert.Error(t, Write(path, nil))
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
