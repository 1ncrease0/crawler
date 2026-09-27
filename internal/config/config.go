package config

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultDepth          = 2
	DefaultTimeout        = 2 * time.Minute
	DefaultRequestTimeout = 10 * time.Second
	DefaultConcurrency    = 10
	MaxConcurrency        = 10
	DefaultOutputPath     = "result.json"
	DefaultLogPath        = "crawler.log"
)

var (
	ErrInvalidConfig = errors.New("invalid config")
	ErrHelp          = flag.ErrHelp
)

type Config struct {
	URLs           []string
	Depth          int
	Timeout        time.Duration
	RequestTimeout time.Duration
	Output         string
	Log            string
	Concurrency    int
}

func Default() Config {
	return Config{
		Depth:          DefaultDepth,
		Timeout:        DefaultTimeout,
		RequestTimeout: DefaultRequestTimeout,
		Output:         DefaultOutputPath,
		Log:            DefaultLogPath,
		Concurrency:    DefaultConcurrency,
	}
}

func (c Config) Validate() error {
	if len(c.URLs) == 0 {
		return fmt.Errorf("%w: at least one URL is required", ErrInvalidConfig)
	}
	if c.Depth < 0 {
		return fmt.Errorf("%w: depth must be >= 0, got %d", ErrInvalidConfig, c.Depth)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("%w: timeout must be > 0, got %s", ErrInvalidConfig, c.Timeout)
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("%w: request timeout must be > 0, got %s", ErrInvalidConfig, c.RequestTimeout)
	}
	if c.Concurrency <= 0 || c.Concurrency > MaxConcurrency {
		return fmt.Errorf("%w: concurrency must be between 1 and %d, got %d", ErrInvalidConfig, MaxConcurrency, c.Concurrency)
	}
	if c.Output == "" {
		return fmt.Errorf("%w: output path must not be empty", ErrInvalidConfig)
	}
	if c.Log == "" {
		return fmt.Errorf("%w: log path must not be empty", ErrInvalidConfig)
	}
	return nil
}

func ParseArgs(args []string) (*Config, error) {
	cfg := Default()

	var rawURLs string
	fs := flag.NewFlagSet("crawler-cli", flag.ContinueOnError)

	fs.StringVar(&rawURLs, "urls", "", "list of starting URLs")
	fs.IntVar(&cfg.Depth, "depth", cfg.Depth, "maxdepth")
	fs.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "overall execution timeout")
	fs.DurationVar(&cfg.RequestTimeout, "request-timeout", cfg.RequestTimeout, "per-request timeout")
	fs.StringVar(&cfg.Output, "output", cfg.Output, "path to the JSON result file")
	fs.StringVar(&cfg.Log, "log", cfg.Log, "path to the log file")
	fs.IntVar(&cfg.Concurrency, "concurrency", cfg.Concurrency, "max number of concurrent requests")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	for _, raw := range strings.Split(rawURLs, ",") {
		if raw = strings.TrimSpace(raw); raw != "" {
			cfg.URLs = append(cfg.URLs, raw)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}
