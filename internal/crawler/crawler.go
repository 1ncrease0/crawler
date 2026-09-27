package crawler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"sync"

	"github.com/1ncrease0/crawler/internal/fetcher"
	"github.com/1ncrease0/crawler/internal/model"
)

type Fetcher interface {
	Fetch(ctx context.Context, target *url.URL) (fetcher.Response, error)
}

type ParseFunc func(r io.Reader, base *url.URL) (model.Page, error)

type Crawler struct {
	fetcher Fetcher
	parse   ParseFunc
	log     *slog.Logger
	depth   int
	workers int
}

func New(f Fetcher, parse ParseFunc, log *slog.Logger, depth, workers int) *Crawler {
	if workers < 1 {
		workers = 1
	}
	return &Crawler{
		fetcher: f,
		parse:   parse,
		log:     log,
		depth:   depth,
		workers: workers,
	}
}

type job struct {
	url    *url.URL
	parent *model.Node
	depth  int
}

type result struct {
	job    job
	status int
	title  string
	links  []string
	err    error
}

func (c *Crawler) Crawl(ctx context.Context, seeds []string) []*model.Node {
	roots := make([]*model.Node, 0, len(seeds))
	visited := make(map[string]struct{}, len(seeds))
	queue := make([]job, 0, len(seeds))

	for _, seed := range seeds {
		u, ok := c.parseLink(seed)
		if !ok {
			continue
		}
		if !markNew(visited, u) {
			continue
		}
		queue = append(queue, job{url: u, depth: 0})
	}

	if len(queue) == 0 {
		return roots
	}

	jobs := make(chan job, c.workers)
	results := make(chan result, c.workers)

	var wg sync.WaitGroup
	for range c.workers {
		wg.Add(1)
		go c.worker(ctx, jobs, results, &wg)
	}

	done := ctx.Done()
	inFlight := 0
	for len(queue) > 0 || inFlight > 0 {
		var send chan job
		var next job
		if len(queue) > 0 {
			send, next = jobs, queue[0]
		}

		select {
		case <-done:
			done = nil
			queue = nil
		case send <- next:
			queue = queue[1:]
			inFlight++
		case res := <-results:
			inFlight--
			queue = append(queue, c.consume(res, visited, &roots)...)
		}
	}

	close(jobs)
	wg.Wait()
	return roots
}

func (c *Crawler) worker(ctx context.Context, jobs <-chan job, results chan<- result, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobs {
		results <- c.loadPage(ctx, j)
	}
}

func (c *Crawler) loadPage(ctx context.Context, j job) result {
	resp, err := c.fetcher.Fetch(ctx, j.url)
	if err != nil {
		status := 0
		if statusErr, ok := errors.AsType[*fetcher.StatusError](err); ok {
			status = statusErr.Code
		}
		return result{job: j, status: status, err: err}
	}
	page, err := c.parse(bytes.NewReader(resp.Body), resp.URL)
	if err != nil {
		return result{job: j, status: resp.StatusCode, err: err}
	}
	return result{job: j, status: resp.StatusCode, title: page.Title, links: page.Links}
}

func (c *Crawler) consume(res result, visited map[string]struct{}, roots *[]*model.Node) []job {
	j := res.job
	if res.err != nil {
		c.log.Warn("fetch failed", "url", j.url.String(), "depth", j.depth, "status", res.status, "err", res.err)
		return nil
	}
	c.log.Info("fetched", "url", j.url.String(), "depth", j.depth, "status", res.status)

	node := &model.Node{Resource: j.url.String(), Title: res.title, Links: []*model.Node{}}
	if j.parent == nil {
		*roots = append(*roots, node)
	} else {
		j.parent.Links = append(j.parent.Links, node)
	}

	if j.depth >= c.depth {
		return nil
	}

	var next []job
	for _, link := range res.links {
		u, err := url.Parse(link)
		if err != nil || !strings.EqualFold(u.Hostname(), j.url.Hostname()) {
			continue
		}
		if !markNew(visited, u) {
			continue
		}
		next = append(next, job{url: u, parent: node, depth: j.depth + 1})
	}
	return next
}

func (c *Crawler) parseLink(seed string) (*url.URL, bool) {
	u, err := url.Parse(seed)
	if err != nil {
		c.log.Warn("invalid seed", "url", seed, "err", err)
		return nil, false
	}
	if u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		c.log.Warn("unsupported seed", "url", seed)
		return nil, false
	}
	return u, true
}

func markNew(visited map[string]struct{}, u *url.URL) bool {
	key := visitKey(u)
	if _, ok := visited[key]; ok {
		return false
	}
	visited[key] = struct{}{}
	return true
}

func visitKey(u *url.URL) string {
	key := *u
	key.Fragment = ""
	key.RawFragment = ""
	if key.Path == "" {
		key.Path = "/"
	}
	return key.String()
}
