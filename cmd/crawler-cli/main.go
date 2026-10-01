package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/1ncrease0/crawler/internal/config"
	"github.com/1ncrease0/crawler/internal/crawler"
	"github.com/1ncrease0/crawler/internal/fetcher"
	"github.com/1ncrease0/crawler/internal/logger"
	"github.com/1ncrease0/crawler/internal/output"
	"github.com/1ncrease0/crawler/internal/parser"
)

func main() {
	cfg, err := config.ParseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, config.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "crawler-cli", err)
		os.Exit(2)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "crawler-cli", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	log, closeLog, err := logger.Setup(cfg.Log)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := closeLog(); cerr != nil {
			fmt.Fprintln(os.Stderr, "crawler-cli: close log:", cerr)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	f := fetcher.New(cfg.RequestTimeout, fetcher.DefaultMaxBodyBytes)
	defer f.Close()

	c := crawler.New(f, parser.ParsePage, log, cfg.Depth, cfg.Concurrency)

	log.Info("crawling started", "urls", len(cfg.URLs), "depth", cfg.Depth, "concurrency", cfg.Concurrency, "output", cfg.Output)
	roots := c.Crawl(ctx, cfg.URLs)

	if err := output.Write(cfg.Output, roots); err != nil {
		return err
	}

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		log.Error("crawling timed out")
		return fmt.Errorf("overall timeout exceeded, partial result saved to %q", cfg.Output)
	case errors.Is(ctx.Err(), context.Canceled):
		log.Info("interrupted, partial result saved")
	default:
		log.Info("crawling finished", "roots", len(roots))
	}
	return nil
}
