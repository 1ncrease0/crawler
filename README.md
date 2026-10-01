# crawler-cli

CLI-краулер: асинхронно обходит стартовые URL, собирает `<title>` и ссылки, сохраняет дерево в JSON. Требования в crawler-cli-test-assignment.md

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
| `--concurrency` | `10` | одновременных запросов (1-10) |

## Результат

Результат выводится в виде дерева страниц в JSON в заданный файл. Ошибки и HTTP-статусы пишутся в лог-файл. При общем таймауте или `Ctrl+C` сохраняется частичный результат.

## Тесты

```bash
make test      
make test-e2e  
```
