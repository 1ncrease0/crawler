BINARY          := bin/crawler-cli
PKG             := ./cmd/crawler-cli

URLS            ?= https://www.youtube.com/
DEPTH           ?= 10
TIMEOUT         ?= 20s
REQUEST_TIMEOUT ?= 10s
OUTPUT          ?= result.json
LOG             ?= crawler.log
CONCURRENCY     ?= 10

.PHONY: build run test

build:
	go build -o $(BINARY) $(PKG)

run:
	go run $(PKG) \
		--urls $(URLS) \
		--depth $(DEPTH) \
		--timeout $(TIMEOUT) \
		--request-timeout $(REQUEST_TIMEOUT) \
		--output $(OUTPUT) \
		--log $(LOG) \
		--concurrency $(CONCURRENCY)

test:
	go test -v -race -count=1 ./...

