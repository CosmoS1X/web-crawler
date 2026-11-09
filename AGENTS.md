# AGENTS.md

## О проекте

Консольный парсер структуры сайта на Go. Модуль — `github.com/CosmoS1X/web-crawler`, бинарник — `web-crawler`.

## Конвенции

- Все комментарии в коде — **на английском**.
- CLI строится на `github.com/urfave/cli/v3`.
- Любая работа с HTTP — только через `*http.Client`, переданный в `Options.HTTPClient`.
  Запрещены `http.Get`, `http.DefaultClient` и неявные клиенты.
- В продакшен-коде `main` создаёт стандартный `&http.Client{}`; тесты передают
  тестовый клиент (`httptest` / кастомный `http.RoundTripper`).
- `Analyze(ctx, Options)` — тонкая точка входа: валидация, запуск обхода, сериализация.
  Состояние обхода (`opts`, `limiter`, `assetCache`) живёт в структуре `crawler`;
  `visited` — локальная карта в `Run`.
- Тесты — во внешних пакетах `_test` (включён линтер `testpackage`), на `testify`:
  `require` для обязательных проверок, `assert` для остальных.
- Тесты `crawler` рассматривают пакет как чёрный ящик: только `Analyze`,
  экспортированные типы/ошибки и переданный `*http.Client`. Запрещены тесты
  в `package crawler`, файлы `*_internal_test.go` и экспорт символов «только для тестов».
- Наблюдать за запросами к тестовому клиенту допустимо только для поведения,
  зафиксированного в `README.md`/`AGENTS.md` (методы, retries, rate limit, кэш,
  User-Agent, лимит тела). Внутренние константы, порядок вызовов и тексты ошибок
  не должны быть предметом проверки, если они не являются публичным контрактом.
- Стремиться к покрытию поведения, включая негативные сценарии и обработку ошибок;
  100% statement coverage не самоцель. Если покрытие участка нецелесообразно
  (glue-код `main`, недостижимые защитные ветки, логика сторонних пакетов),
  явно указывать это с обоснованием в отчёте о задаче.
- Сетевые ошибки не считаются ошибкой запуска: они попадают в отчёт, код выхода — 0.
  Код 1 — при некорректных аргументах/опциях и ошибках записи отчёта.
- Линтер: `golangci-lint` (`2.15.0`), конфиг `.golangci.yml`. При ложном срабатывании
  сначала обсуждаем и правим конфиг, а не замалчиваем код. При переименовании модуля
  обновлять `gofumpt.module-path` в `.golangci.yml`.
- `gocritic:hugeParam` отключён в `.golangci.yml`, так как `Options`
  передаётся по значению.
- Makefile: `build` (с версией из `git describe`), `test`, `test-race`, `cover`
  (профиль + сводка), `cover-show` (открыть отчёт в браузере), `run`, `fmt`,
  `lint` (зависит от `fmt`), `tidy`, `clean`, `tag` / `tag-push` / `release`
  (создание и отправка тега версии), `snapshot` (локальная сборка GoReleaser).
- Перед завершением задачи обязательно: `make test && make lint`.

## Коммиты

- Коммиты делаются только по явному запросу.
- Conventional Commits: `feat:`, `fix:`, `refactor:`, `chore:`, `test:`, `docs:`, `ci:`.
- Commit-сообщения — на английском.
- Коммиты небольшие и логически раздельные; каждый коммит должен собираться
  (`go build ./...` или `make build`).
- Примеры: `feat: add depth flag`, `fix: keep exit code zero on network errors`,
  `refactor: extract fetch logic into fetch.go`, `test: cover invalid urls`,
  `docs: describe commit conventions`.

## Целевая структура

```
.
├── cmd/web-crawler/
│   └── main.go              # создаёт &http.Client{}, версия из ldflags, запускает cli.Command
├── internal/app/
│   ├── app.go               # urfave/cli: аргументы/флаги -> crawler.Options
│   └── app_test.go
├── crawler/
│   ├── analyze.go           # Analyze(ctx, Options) ([]byte, error)
│   ├── options.go           # Options, sentinel-ошибки, валидация
│   ├── crawler.go           # структура обхода, Run/visitPage, BFS
│   ├── fetch.go             # fetch/doRequest, режимы чтения, charset, лимит тела, расчёт размера ассета, повторы, rate limiting и timeout попытки
│   ├── links.go             # типизированное извлечение ссылок и проверка битых ссылок
│   ├── assets.go            # проверка ассетов, размеры и кэш
│   ├── seo.go               # SEO-метрики: title, description, h1
│   ├── report.go            # типы отчёта, SEO и ассетов, JSON-сериализация
│   ├── url.go               # нормализация URL, host, base href
│   ├── testdata/            # HTML- и JSON-фикстуры
│   └── *_test.go            # analyze/options/fetch/url/links/assets/crawler/seo/report + helpers
├── .github/workflows/
│   ├── ci.yml               # lint, format, race-тесты, build, проверка GoReleaser
│   └── release.yml          # сборка релиза по тегу v*
├── .goreleaser.yaml
├── Makefile
├── README.md
├── AGENTS.md
├── .gitignore
└── .golangci.yml
```

## Публичный API

```go
func Analyze(ctx context.Context, opts Options) ([]byte, error)
```

Sentinel-ошибки: `ErrInvalidURL`, `ErrNilHTTPClient`, `ErrInvalidOptions`.

## Options

Поля:

- `URL string`
- `Depth int`
- `Timeout time.Duration`
- `IndentJSON bool`
- `HTTPClient *http.Client`
- `Retries int`
- `Delay time.Duration`
- `UserAgent string`
- `Concurrency int` (зарезервировано, пока не влияет на обход)
- `RPS float64` (приоритет над `Delay`)

Валидация: отрицательные `Depth`, `Timeout`, `Retries`, `Delay`, `Concurrency`, `RPS` возвращают `ErrInvalidOptions`. `HTTPClient` обязателен, `URL` проверяется как HTTP(S).

## JSON-отчёт

```json
{
  "root_url": "https://example.com",
  "depth": 1,
  "generated_at": "2026-10-22T12:00:00Z",
  "pages": [
    {
      "url": "https://example.com",
      "depth": 0,
      "http_status": 200,
      "status": "ok",
      "seo": {
        "has_title": true,
        "title": "Example Test",
        "has_description": true,
        "description": "A description",
        "has_h1": true
      },
      "broken_links": [
        {
          "url": "https://example.com/missing",
          "status_code": 404,
          "error": "Not Found"
        },
        {
          "url": "https://offline.example/page",
          "error": "Get \"https://offline.example/page\": dial tcp: lookup offline.example: no such host"
        }
      ],
      "assets": [
        {
          "url": "https://example.com/static/logo.png",
          "type": "image",
          "size_bytes": 12345,
          "status_code": 200
        }
      ],
      "discovered_at": "2026-10-22T12:00:00Z"
    }
  ]
}
```

- Обязательные ключи: `root_url`, `depth`, `generated_at`, `pages`; у страницы — `url`, `depth`, `http_status`, `status`, `seo`, `broken_links`, `assets`, `discovered_at`.
- `error` у страницы опускается при успехе (пустая строка). У `broken_links` и `assets` опускаются пустой `error`, нулевые `status_code` и `size_bytes`.
- `generated_at` — UTC с точностью до секунды, формат ISO8601/RFC3339.
- `discovered_at` — UTC с точностью до секунды, формат ISO8601/RFC3339.
- `depth` — число уровней обхода, включая корень. `Depth: 1` означает только корневую страницу; `Depth: 0` — корень без обхода ссылок.
- `pages` сортируется по `depth`, затем по `url`.
- `status`: `ok`, если запрос успешен и HTTP < 400; иначе `error`.
- `http_status`: 0, если ответ не получен; при ошибке чтения тела сохраняется полученный код ответа.
- При сетевой ошибке/ошибке чтения тела страницы `broken_links` и `assets` равны `null`, SEO остаётся нулевым.
- `seo` — базовые SEO-метрики страницы: `has_title`/`title`, `has_description`/`description`, `has_h1`.
- `has_*`: `true`, если соответствующий тег найден; иначе `false` и пустое текстовое поле.
- Текст `title` и `description` — очищенный (лишние пробелы схлопнуты) и с раскодированными HTML-сущностями. Для `title` берётся только первый тег.
- SEO вычисляется для страниц с полученным телом, включая HTTP-ошибки; при сетевой ошибке блок `seo` остаётся нулевым.
- `broken_links` — массив; содержит только навигационные ссылки `a[href]` с HTTP 4xx/5xx или сетевой ошибкой.
- Для HTTP-ошибки заполняются `status_code` и `error` (текст статуса, например `"Not Found"`), для сетевой — только текст ошибки.
- `assets` — массив; содержит `img[src]`, `script[src]` и `link[href]` с `rel~="stylesheet"`. Прочие `link` игнорируются.
- `type`: `image`, `script` или `style`; `other` не используется.
- Порядок ассетов: изображения, затем скрипты, затем стили; повторяющиеся пары `type + URL` схлопываются.
- `size_bytes`: из `Content-Length`, если он есть; иначе по фактически прочитанному телу. При ошибке запроса ключ опускается.
- HTTP-ошибка ассета: заполняются `status_code` и `error` (текст статуса), `size_bytes` опускается.
- Ссылки и ассеты проверяются запросом `HEAD`; при `405` выполняется повторный `GET`. Для ассетов `GET` также используется, если `HEAD` вернул неизвестный `Content-Length`. Страницы загружаются `GET`.
- Внутренние ссылки, которые всё равно будут обойдены на следующем уровне, отдельно не проверяются; ссылки на текущую страницу пропускаются.
- Ассеты кэшируются на время запуска по абсолютному URL без fragment: повторный ассет не запрашивается снова, а его данные в отчётах совпадают.
- URL в `broken_links` и `assets` абсолютный.
- Относительные ссылки и ассеты разрешаются относительно первого валидного `<base href>` страницы, иначе относительно URL страницы.
- Тело страницы читается не более 10 MiB; байты сверх лимита игнорируются.
- Кодировка страницы определяется по BOM, `Content-Type` или `<meta charset>` и приводится к UTF-8.
- Пустые значения, `#...`, не-HTTP(S) схемы, URL без host и невалидные URL игнорируются.
- `IndentJSON` влияет только на форматирование (пробелы/переносы), но не на состав и порядок ключей.

## CLI

`web-crawler <url>` (обязательный аргумент) + `--help`, `--version`.

Флаги:

| Флаг | Тип | По умолчанию | Описание |
| --- | --- | --- | --- |
| `--depth` | int | 10 | crawl depth |
| `--retries` | int | 1 | number of retries for failed requests |
| `--delay` | duration | 0s | delay between requests (example: 200ms, 1s) |
| `--timeout` | duration | 15s | per-request timeout |
| `--rps` | float | 0 | limit requests per second (overrides delay) |
| `--user-agent` | string | "" | custom user agent |
| `--version` | flag | | print the version and exit |

`--version` печатает `web-crawler version <version>`. Версия задаётся при сборке через `-ldflags "-X main.version=..."`; в локальной сборке без тега — `dev`, в `make build` — значение `git describe`.

`RPS` имеет приоритет над `Delay`. Глобальное ограничение частоты применяется ко всем HTTP-запросам: страницы, проверки ссылок и ассетов. `User-Agent` добавляется к запросам, если указан. При отмене контекста ожидание rate-лимитера прерывается без зависания.

`Retries` задаёт максимальное число повторных попыток (всего до `Retries + 1` запросов). Повторяются только временные сбои: сетевые ошибки, ошибки чтения тела, `429` и `5xx`; успех прекращает повторы. Перед каждой повторной попыткой выдерживается ненулевая пауза, прерываемая отменой контекста. В отчёт попадает результат последней попытки.

CLI печатает отчёт без изменений: ровно JSON и один завершающий перевод строки, без текста до или после.

## Релизы

- Релиз собирается GoReleaser (`.goreleaser.yaml`) в workflow `.github/workflows/release.yml` по пушу тега `v*`.
- Артефакты: `linux`, `darwin`, `windows` × `amd64`, `arm64`; архивы `tar.gz` (`zip` для Windows), `checksums.txt`, GitHub Release.
- Версия передаётся в бинарник через `-ldflags "-X main.version={{.Version}}"`.
- Теги создаются через `make tag VERSION=1.2.3` (проверяет semver, чистоту дерева и отсутствие тега), отправляются через `make tag-push VERSION=1.2.3` или одной командой `make release VERSION=1.2.3`.
- Локальная проверка конфигурации: `goreleaser check` и `make snapshot`.
- CI дополнительно выполняет `goreleaser check`, чтобы конфиг не ломался до релиза.
