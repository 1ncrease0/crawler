package fetcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

const (
	DefaultMaxBodyBytes int64 = 10 << 20
	userAgent                 = "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:15.0) Gecko/20100101 Firefox/15.0.1"
)

var (
	ErrNotHTML      = errors.New("response is not html")
	ErrBodyTooLarge = errors.New("response body too large")
)

type StatusError struct {
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected http status: %d %s", e.Code, http.StatusText(e.Code))
}

type Response struct {
	URL        *url.URL
	StatusCode int
	Body       []byte
}

type Fetcher struct {
	client         *http.Client
	requestTimeout time.Duration
	maxBodyBytes   int64
}

func New(requestTimeout time.Duration, maxBodyBytes int64) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		requestTimeout: requestTimeout,
		maxBodyBytes:   maxBodyBytes,
	}
}

func (f *Fetcher) Close() {
	f.client.CloseIdleConnections()
}

func (f *Fetcher) Fetch(ctx context.Context, target *url.URL) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, f.requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return Response{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := f.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("request %s: %w", target, err)
	}
	defer func() { _ = resp.Body.Close() }()

	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, &StatusError{Code: resp.StatusCode}
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBodyBytes+1))
	if err != nil {
		return Response{}, fmt.Errorf("read body %s: %w", target, err)
	}
	if int64(len(raw)) > f.maxBodyBytes {
		return Response{}, ErrBodyTooLarge
	}

	if !isHTML(contentType, raw) {
		return Response{}, fmt.Errorf("%w: %s", ErrNotHTML, contentType)
	}

	decoded, err := charset.NewReader(bytes.NewReader(raw), contentType)
	if err != nil {
		return Response{}, fmt.Errorf("decode body %s: %w", target, err)
	}

	body, err := io.ReadAll(decoded)
	if err != nil {
		return Response{}, fmt.Errorf("read decoded body %s: %w", target, err)
	}

	return Response{URL: target, StatusCode: resp.StatusCode, Body: body}, nil
}

func isHTML(contentType string, raw []byte) bool {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType == "" {
		mediaType, _, _ = strings.Cut(http.DetectContentType(raw), ";")
	}

	switch mediaType {
	case "text/html", "application/xhtml+xml":
		return true
	default:
		return false
	}
}
