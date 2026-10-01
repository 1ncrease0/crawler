package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/1ncrease0/crawler/internal/model"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "crawler-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: create temp dir:", err)
		os.Exit(1)
	}
	binPath = filepath.Join(dir, "crawler-cli")

	build := exec.Command("go", "build", "-o", binPath, "./cmd/crawler-cli")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: build failed: %v\n%s", err, out)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestCLICrawl(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><title>Root Page</title></head><body><a href="%s/a">a</a></body></html>`, srv.URL)
	})
	mux.HandleFunc("/a", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Page A</title></head></html>`)
	})

	dir := t.TempDir()
	outPath := filepath.Join(dir, "result.json")
	logPath := filepath.Join(dir, "crawler.log")

	code, output := runCLI(t,
		"--urls", srv.URL,
		"--depth", "1",
		"--timeout", "20s",
		"--request-timeout", "5s",
		"--output", outPath,
		"--log", logPath,
	)

	require.Equal(t, 0, code, "cli output: %s", output)

	var roots []*model.Node
	require.NoError(t, json.Unmarshal(readFile(t, outPath), &roots))
	require.Len(t, roots, 1)
	assert.Equal(t, srv.URL, roots[0].Resource)
	assert.Equal(t, "Root Page", roots[0].Title)
	require.Len(t, roots[0].Links, 1)
	assert.Equal(t, "Page A", roots[0].Links[0].Title)

	logs := string(readFile(t, logPath))
	assert.Contains(t, logs, "crawling finished")
	assert.Contains(t, logs, "status=200")
}

func TestCLIInvalidConfig(t *testing.T) {
	t.Parallel()

	code, output := runCLI(t, "--urls", "https://example.com", "--concurrency", "50")

	assert.Equal(t, 2, code)
	assert.Contains(t, output, "invalid config")
}

func TestCLIGracefulShutdown(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	var once sync.Once

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Root</title></head><body><a href="/slow">slow</a></body></html>`)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		<-r.Context().Done()
	})

	dir := t.TempDir()
	outPath := filepath.Join(dir, "result.json")
	logPath := filepath.Join(dir, "crawler.log")

	cmd := exec.Command(binPath,
		"--urls", srv.URL,
		"--depth", "2",
		"--timeout", "60s",
		"--request-timeout", "30s",
		"--output", outPath,
		"--log", logPath,
	)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("crawler did not request the slow page")
	}

	require.NoError(t, cmd.Process.Signal(os.Interrupt))
	require.NoError(t, waitWithTimeout(cmd, 10*time.Second), "graceful shutdown must exit 0")

	logs := string(readFile(t, logPath))
	assert.Contains(t, logs, "interrupted, partial result saved")

	var roots []*model.Node
	require.NoError(t, json.Unmarshal(readFile(t, outPath), &roots))
	assert.Len(t, roots, 1)
}

func TestCLIOverallTimeout(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Root</title></head><body><a href="/slow">slow</a></body></html>`)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	dir := t.TempDir()
	outPath := filepath.Join(dir, "result.json")
	logPath := filepath.Join(dir, "crawler.log")

	code, output := runCLI(t,
		"--urls", srv.URL,
		"--depth", "2",
		"--timeout", "2s",
		"--request-timeout", "30s",
		"--output", outPath,
		"--log", logPath,
	)

	assert.Equal(t, 1, code, "cli output: %s", output)
	assert.Contains(t, output, "overall timeout exceeded")

	var roots []*model.Node
	require.NoError(t, json.Unmarshal(readFile(t, outPath), &roots))
	assert.Len(t, roots, 1)

	logs := string(readFile(t, logPath))
	assert.Contains(t, logs, "crawling timed out")
}

func runCLI(t *testing.T, args ...string) (int, string) {
	t.Helper()

	out, err := exec.Command(binPath, args...).CombinedOutput()
	if err == nil {
		return 0, string(out)
	}

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	return exitErr.ExitCode(), string(out)
}

func waitWithTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("process did not exit within %s", timeout)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
