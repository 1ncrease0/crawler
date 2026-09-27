package fetcher

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testHTML = `<html><head><title>Hi</title></head><body>ok</body></html>`

func newTestFetcher(t *testing.T, timeout time.Duration, maxBody int64) *Fetcher {
	t.Helper()
	f := New(timeout, maxBody)
	t.Cleanup(f.Close)
	return f
}

func serveHTML(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchSuccess(t *testing.T) {
	t.Parallel()

	srv := serveHTML(t, testHTML)
	target, err := url.Parse(srv.URL)
	require.NoError(t, err)

	resp, err := newTestFetcher(t, time.Second, DefaultMaxBodyBytes).Fetch(context.Background(), target)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Same(t, target, resp.URL)
	assert.Equal(t, testHTML, string(resp.Body))
}

func TestFetchSkipped(t *testing.T) {
	t.Parallel()

	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

	tests := []struct {
		name        string
		contentType string
		status      int
		body        []byte
		maxBody     int64
		wantStatus  int
		wantErr     error
	}{
		{name: "not found", contentType: "text/html", status: http.StatusNotFound, body: []byte(testHTML), wantStatus: http.StatusNotFound},
		{name: "server error", contentType: "text/html", status: http.StatusInternalServerError, body: []byte(testHTML), wantStatus: http.StatusInternalServerError},
		{name: "plain text", contentType: "text/plain", body: []byte(testHTML), wantErr: ErrNotHTML},
		{name: "json", contentType: "application/json", body: []byte(`{"a":1}`), wantErr: ErrNotHTML},
		{name: "image", contentType: "image/png", body: png, wantErr: ErrNotHTML},
		{name: "too large", contentType: "text/html", body: bytes.Repeat([]byte("a"), 100), maxBody: 10, wantErr: ErrBodyTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}
				if tt.status != 0 {
					w.WriteHeader(tt.status)
				}
				_, _ = w.Write(tt.body)
			}))
			t.Cleanup(srv.Close)

			maxBody := tt.maxBody
			if maxBody == 0 {
				maxBody = DefaultMaxBodyBytes
			}
			target, err := url.Parse(srv.URL)
			require.NoError(t, err)
			resp, err := newTestFetcher(t, time.Second, maxBody).Fetch(context.Background(), target)

			require.Error(t, err)
			assert.Empty(t, resp.Body)
			if tt.wantStatus != 0 {
				var statusErr *StatusError
				require.ErrorAs(t, err, &statusErr)
				assert.Equal(t, tt.wantStatus, statusErr.Code)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestFetchNotFollowRedirects(t *testing.T) {
	t.Parallel()

	var finalHits atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final", http.StatusFound)
	})
	mux.HandleFunc("/final", func(w http.ResponseWriter, _ *http.Request) {
		finalHits.Add(1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, testHTML)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL + "/start")
	require.NoError(t, err)
	_, err = newTestFetcher(t, time.Second, DefaultMaxBodyBytes).Fetch(context.Background(), target)

	var statusErr *StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, http.StatusFound, statusErr.Code)
	assert.Zero(t, finalHits.Load(), "redirect target must not be requested")
}

func TestFetchTimeout(t *testing.T) {
	t.Parallel()

	slow := func(t *testing.T) *httptest.Server {
		t.Helper()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(500 * time.Millisecond)
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, testHTML)
		}))
		t.Cleanup(srv.Close)
		return srv
	}

	t.Run("per-request deadline", func(t *testing.T) {
		t.Parallel()

		target, err := url.Parse(slow(t).URL)
		require.NoError(t, err)
		start := time.Now()
		_, err = newTestFetcher(t, 50*time.Millisecond, DefaultMaxBodyBytes).Fetch(context.Background(), target)

		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(start), 400*time.Millisecond)
	})

	t.Run("caller cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		target, err := url.Parse(slow(t).URL)
		require.NoError(t, err)
		_, err = newTestFetcher(t, time.Second, DefaultMaxBodyBytes).Fetch(ctx, target)

		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestIsHTML(t *testing.T) {
	t.Parallel()

	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

	tests := []struct {
		name        string
		contentType string
		raw         []byte
		want        bool
	}{
		{name: "text/html", contentType: "text/html", raw: []byte(testHTML), want: true},
		{name: "empty sniff html", contentType: "", raw: []byte(testHTML), want: true},
		{name: "empty sniff binary", contentType: "", raw: png, want: false},
		{name: "plain text", contentType: "text/plain", raw: []byte(testHTML), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isHTML(tt.contentType, tt.raw))
		})
	}
}
