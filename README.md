# crawler-cli

CLI-краулер: асинхронно обходит стартовыe URL, собирает `<title>` и ссылки, сохраняет дерево в JSON, требования в crawler-cli-test-assignment.md

## Сборка

```bash
make build
go build -o bin/crawler-cli ./cmd/crawler-cli
```

## Запуск

```bash
./bin/crawler-cli \
  --urls https://google.com,https://example.com \
  --depth 3 \
  --timeout 2m \
  --request-timeout 10s \
  --output result.json \
  --log crawler.log
```

Или через `make`, параметры переопределяются:

```bash
make run
make run URLS=https://metanit.com DEPTH=2
```

## Флаги

| Флаг | По умолчанию | Описание |
|---|---|---|
| `--urls` | - | список стартовых URL через запятую |
| `--depth` | `2` | максимальная глубина обхода |
| `--timeout` | `2m` | общий таймаут выполнения |
| `--request-timeout` | `10s` | таймаут одного запроса |
| `--output` | `result.json` | файл с результатом |
| `--log` | `crawler.log` | файл логов |
| `--concurrency` | `10` | одновременных запросов |

## Результат

Результат выводится в виде дерева страниц в JSON в заданный файл. Также согласно требованию записывается лог-файл