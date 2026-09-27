package crawler_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/1ncrease0/crawler/internal/crawler"
	"github.com/1ncrease0/crawler/internal/fetcher"
	"github.com/1ncrease0/crawler/internal/model"
)

func TestCrawlBuildsTree(t *testing.T) {
	t.Parallel()

	pages := map[string]model.Page{
		"http://site/":  {Title: "Root", Links: []string{"http://site/a", "http://site/b", "http://other/x"}},
		"http://site/a": {Title: "A", Links: []string{"http://site/", "http://site/c"}},
		"http://site/b": {Title: "B"},
		"http://site/c": {Title: "C"},
	}
	f, calls := countingCalls(success)
	c := newCrawler(t, f, pages, 3, 4)

	roots := c.Crawl(context.Background(), []string{"http://site/"})

	require.Len(t, roots, 1)
	titles, children := flatten(roots)
	assert.Equal(t, map[string]string{
		"http://site/":  "Root",
		"http://site/a": "A",
		"http://site/b": "B",
		"http://site/c": "C",
	}, titles)
	assert.ElementsMatch(t, []string{"http://site/a", "http://site/b"}, children["http://site/"])
	assert.ElementsMatch(t, []string{"http://site/c"}, children["http://site/a"])
	assert.Equal(t, int64(4), calls.Load(), "external domain and visited urls must not be fetched")
}

func TestCrawlRespectsDepth(t *testing.T) {
	t.Parallel()

	pages := map[string]model.Page{
		"http://site/":  {Title: "root", Links: []string{"http://site/a"}},
		"http://site/a": {Title: "a", Links: []string{"http://site/b"}},
		"http://site/b": {Title: "b", Links: []string{"http://site/c"}},
		"http://site/c": {Title: "c"},
	}

	tests := []struct {
		depth int
		want  []string
	}{
		{depth: 0, want: []string{"http://site/"}},
		{depth: 1, want: []string{"http://site/", "http://site/a"}},
		{depth: 2, want: []string{"http://site/", "http://site/a", "http://site/b"}},
		{depth: 3, want: []string{"http://site/", "http://site/a", "http://site/b", "http://site/c"}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("depth %d", tt.depth), func(t *testing.T) {
			t.Parallel()

			c := newCrawler(t, fetchFunc(success), pages, tt.depth, 2)
			roots := c.Crawl(context.Background(), []string{"http://site/"})

			titles, _ := flatten(roots)
			assert.ElementsMatch(t, tt.want, keys(titles))
		})
	}
}

func TestCrawlSkipsVisited(t *testing.T) {
	t.Parallel()

	t.Run("trailing slash and fragment", func(t *testing.T) {
		t.Parallel()

		pages := map[string]model.Page{
			"http://site": {Title: "Root", Links: []string{"http://site/", "http://site#top"}},
		}
		f, calls := countingCalls(success)
		c := newCrawler(t, f, pages, 3, 2)

		roots := c.Crawl(context.Background(), []string{"http://site"})

		require.Len(t, roots, 1)
		assert.Empty(t, roots[0].Links)
		assert.Equal(t, int64(1), calls.Load())
	})

	t.Run("cycle", func(t *testing.T) {
		t.Parallel()

		pages := map[string]model.Page{
			"http://site/":  {Title: "Root", Links: []string{"http://site/a"}},
			"http://site/a": {Title: "A", Links: []string{"http://site/"}},
		}
		f, calls := countingCalls(success)
		c := newCrawler(t, f, pages, 3, 2)

		roots := c.Crawl(context.Background(), []string{"http://site/"})

		_, children := flatten(roots)
		assert.Equal(t, int64(2), calls.Load())
		assert.Empty(t, children["http://site/a"])
	})
}

func TestCrawlContinuesOnError(t *testing.T) {
	t.Parallel()

	t.Run("failed link is skipped", func(t *testing.T) {
		t.Parallel()

		pages := map[string]model.Page{
			"http://site/":  {Title: "Root", Links: []string{"http://site/a", "http://site/b"}},
			"http://site/a": {Title: "A"},
			"http://site/b": {Title: "B"},
		}
		f := fetchFunc(func(ctx context.Context, u *url.URL) (fetcher.Response, error) {
			if u.String() == "http://site/a" {
				return fetcher.Response{}, errors.New("boom")
			}
			return success(ctx, u)
		})
		c := newCrawler(t, f, pages, 3, 2)

		roots := c.Crawl(context.Background(), []string{"http://site/"})

		require.Len(t, roots, 1)
		titles, children := flatten(roots)
		assert.Equal(t, map[string]string{"http://site/": "Root", "http://site/b": "B"}, titles)
		assert.ElementsMatch(t, []string{"http://site/b"}, children["http://site/"])
	})

	t.Run("failed seed is dropped", func(t *testing.T) {
		t.Parallel()

		f := fetchFunc(func(context.Context, *url.URL) (fetcher.Response, error) {
			return fetcher.Response{}, errors.New("boom")
		})
		c := newCrawler(t, f, nil, 3, 2)

		roots := c.Crawl(context.Background(), []string{"http://site/"})

		assert.Empty(t, roots)
	})
}

func TestCrawlContextCancel(t *testing.T) {
	t.Parallel()

	pages := map[string]model.Page{
		"http://site/":  {Title: "Root", Links: []string{"http://site/a"}},
		"http://site/a": {Title: "A"},
	}
	f := fetchFunc(func(ctx context.Context, u *url.URL) (fetcher.Response, error) {
		if u.String() == "http://site/a" {
			<-ctx.Done()
			return fetcher.Response{}, ctx.Err()
		}
		return success(ctx, u)
	})
	c := newCrawler(t, f, pages, 3, 2)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	done := make(chan []*model.Node, 1)
	go func() { done <- c.Crawl(ctx, []string{"http://site/"}) }()

	select {
	case roots := <-done:
		assert.Less(t, time.Since(start), 2*time.Second)
		require.Len(t, roots, 1)
		assert.Equal(t, "Root", roots[0].Title)
		assert.Empty(t, roots[0].Links)
	case <-time.After(2 * time.Second):
		t.Fatal("Crawl did not stop after context cancellation")
	}
}

func TestCrawlLimitsConcurrency(t *testing.T) {
	t.Parallel()

	var calls, active, maxActive atomic.Int64
	f := fetchFunc(func(ctx context.Context, u *url.URL) (fetcher.Response, error) {
		calls.Add(1)
		cur := active.Add(1)
		for {
			max := maxActive.Load()
			if cur <= max || maxActive.CompareAndSwap(max, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		return success(ctx, u)
	})
	c := newCrawler(t, f, nil, 0, 3)

	seeds := make([]string, 20)
	for i := range seeds {
		seeds[i] = fmt.Sprintf("http://site/%d", i)
	}

	c.Crawl(context.Background(), seeds)

	assert.Equal(t, int64(20), calls.Load())
	assert.LessOrEqual(t, maxActive.Load(), int64(3))
	assert.Greater(t, maxActive.Load(), int64(1))
}

func TestCrawlSkipsInvalidSeeds(t *testing.T) {
	t.Parallel()

	f, calls := countingCalls(success)
	c := newCrawler(t, f, nil, 3, 2)

	roots := c.Crawl(context.Background(), []string{"", "not-a-url", "ftp://site/", "http://", "mailto:a@example.com"})

	assert.Empty(t, roots)
	assert.Zero(t, calls.Load())
}

type fetchFunc func(ctx context.Context, u *url.URL) (fetcher.Response, error)

func (f fetchFunc) Fetch(ctx context.Context, u *url.URL) (fetcher.Response, error) {
	return f(ctx, u)
}

func success(_ context.Context, u *url.URL) (fetcher.Response, error) {
	return fetcher.Response{URL: u, StatusCode: http.StatusOK}, nil
}

func countingCalls(base fetchFunc) (crawler.Fetcher, *atomic.Int64) {
	var calls atomic.Int64
	return fetchFunc(func(ctx context.Context, u *url.URL) (fetcher.Response, error) {
		calls.Add(1)
		return base(ctx, u)
	}), &calls
}

func newCrawler(t *testing.T, f crawler.Fetcher, pages map[string]model.Page, depth, workers int) *crawler.Crawler {
	t.Helper()
	parse := func(_ io.Reader, base *url.URL) (model.Page, error) {
		return pages[base.String()], nil
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return crawler.New(f, parse, log, depth, workers)
}

func flatten(roots []*model.Node) (map[string]string, map[string][]string) {
	titles := map[string]string{}
	children := map[string][]string{}
	var walk func(n *model.Node)
	walk = func(n *model.Node) {
		titles[n.Resource] = n.Title
		for _, c := range n.Links {
			children[n.Resource] = append(children[n.Resource], c.Resource)
			walk(c)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return titles, children
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
