package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	base := func() Config {
		c := Default()
		c.URLs = []string{"https://example.com"}
		return c
	}

	tests := []struct {
		name    string
		mutate  func(c *Config)
		wantErr bool
	}{
		{name: "valid default", mutate: func(*Config) {}},
		{name: "no urls", mutate: func(c *Config) { c.URLs = nil }, wantErr: true},
		{name: "negative depth", mutate: func(c *Config) { c.Depth = -1 }, wantErr: true},
		{name: "zero timeout", mutate: func(c *Config) { c.Timeout = 0 }, wantErr: true},
		{name: "zero request timeout", mutate: func(c *Config) { c.RequestTimeout = 0 }, wantErr: true},
		{name: "concurrency zero", mutate: func(c *Config) { c.Concurrency = 0 }, wantErr: true},
		{name: "concurrency above max", mutate: func(c *Config) { c.Concurrency = MaxConcurrency + 1 }, wantErr: true},
		{name: "empty output", mutate: func(c *Config) { c.Output = "" }, wantErr: true},
		{name: "empty log", mutate: func(c *Config) { c.Log = "" }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := base()
			tt.mutate(&cfg)

			err := cfg.Validate()
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrInvalidConfig)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestParseArgs(t *testing.T) {
	t.Parallel()

	t.Run("all flags", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{
			"--urls", "https://a.com, https://b.com",
			"--depth", "3",
			"--timeout", "30s",
			"--request-timeout", "5s",
			"--output", "out.json",
			"--log", "log.txt",
			"--concurrency", "4",
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"https://a.com", "https://b.com"}, cfg.URLs)
		assert.Equal(t, 3, cfg.Depth)
		assert.Equal(t, 30*time.Second, cfg.Timeout)
		assert.Equal(t, 5*time.Second, cfg.RequestTimeout)
		assert.Equal(t, "out.json", cfg.Output)
		assert.Equal(t, "log.txt", cfg.Log)
		assert.Equal(t, 4, cfg.Concurrency)
	})

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{"--urls", "https://a.com"})

		require.NoError(t, err)
		assert.Equal(t, []string{"https://a.com"}, cfg.URLs)
		assert.Equal(t, DefaultDepth, cfg.Depth)
		assert.Equal(t, DefaultTimeout, cfg.Timeout)
		assert.Equal(t, DefaultRequestTimeout, cfg.RequestTimeout)
		assert.Equal(t, DefaultOutputPath, cfg.Output)
		assert.Equal(t, DefaultLogPath, cfg.Log)
		assert.Equal(t, DefaultConcurrency, cfg.Concurrency)
	})

	t.Run("empty entries are ignored", func(t *testing.T) {
		t.Parallel()

		cfg, err := ParseArgs([]string{"--urls", " https://a.com ,, "})

		require.NoError(t, err)
		assert.Equal(t, []string{"https://a.com"}, cfg.URLs)
	})

	t.Run("missing urls", func(t *testing.T) {
		t.Parallel()

		_, err := ParseArgs(nil)

		assert.ErrorIs(t, err, ErrInvalidConfig)
	})
}
