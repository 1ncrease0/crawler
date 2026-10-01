package logger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crawler.log")

	log, closeLog, err := Setup(path)

	require.NoError(t, err)
	require.NotNil(t, log)
	require.NotNil(t, closeLog)

	log.Info("hello", "answer", 42)
	require.NoError(t, closeLog())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "level=INFO")
	assert.Contains(t, string(data), "msg=hello")
	assert.Contains(t, string(data), "answer=42")
}

func TestSetupError(t *testing.T) {
	t.Parallel()

	log, closeLog, err := Setup(filepath.Join(t.TempDir(), "missing", "crawler.log"))

	assert.Error(t, err)
	assert.Nil(t, log)
	assert.Nil(t, closeLog)
}
