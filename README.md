# web-crawler

[![CI](https://github.com/CosmoS1X/web-crawler/actions/workflows/ci.yml/badge.svg)](https://github.com/CosmoS1X/web-crawler/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/CosmoS1X/web-crawler?display_name=tag)](https://github.com/CosmoS1X/web-crawler/releases)

`web-crawler` is a command-line website crawler written in Go. It walks a site
starting from a root URL and prints a single JSON report with page statuses,
basic SEO metrics, broken navigation links and asset sizes.

## Features

- Breadth-first crawl with a configurable depth limit.
- Broken link detection for `a[href]` navigation links (HTTP 4xx/5xx and
  network errors).
- Asset inventory for images, scripts and stylesheets, with sizes and status
  codes.
- Basic SEO metrics: `title`, `description` and `h1`.
- Relative URL resolution, including the first valid `<base href>`.
- Charset detection from BOM, `Content-Type` or `<meta charset>`, converted to
  UTF-8.
- Page body limit of 10 MiB.
- Retries for transient failures, global rate limiting and a custom User-Agent.
- A stable, sorted JSON report suitable for diffing and automation.

## Requirements

- Go 1.27.3 or newer to build from source.
- No runtime dependencies: the released binaries are statically linked.

## Installation

### Prebuilt binaries

Download the archive for your OS and architecture from
[Releases](https://github.com/CosmoS1X/web-crawler/releases), unpack it and put
the `web-crawler` binary somewhere in your `PATH`.

### With `go install`

```sh
go install github.com/CosmoS1X/web-crawler/cmd/web-crawler@latest
```

### From source

```sh
git clone git@github.com:CosmoS1X/web-crawler.git
cd web-crawler
make build
./bin/web-crawler --version
```

## Usage

```sh
web-crawler [options] <url>
```

The URL is required and must use the `http` or `https` scheme. The report is
written to stdout as JSON followed by a single newline; diagnostics go to
stderr.

```sh
web-crawler https://example.com --depth 2 --delay 100ms
```

### Flags

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--depth` | int | `10` | crawl depth |
| `--retries` | int | `1` | number of retries for failed requests |
| `--delay` | duration | `0s` | delay between requests (for example `200ms`, `1s`) |
| `--timeout` | duration | `15s` | per-request timeout |
| `--rps` | float | `0` | limit requests per second (overrides `--delay`) |
| `--user-agent` | string | `""` | custom User-Agent header |
| `--version` | | | print the version and exit |
| `--help` | | | print help and exit |

`--rps` takes precedence over `--delay`. The rate limit is global and applies to
every HTTP request: pages, link checks and assets. `--retries` sets the maximum
number of retries, so a request is attempted at most `Retries + 1` times; only
network errors, body read errors, `429` and `5xx` responses are retried.

### Example

```sh
$ web-crawler https://example.com --depth 1
{
  "root_url": "https://example.com",
  "depth": 1,
  "generated_at": "2026-11-04T12:00:00Z",
  "pages": [
    {
      "url": "https://example.com",
      "depth": 0,
      "http_status": 200,
      "status": "ok",
      "seo": {
        "has_title": true,
        "title": "Example Domain",
        "has_description": true,
        "description": "Example description",
        "has_h1": true
      },
      "broken_links": [
        {
          "url": "https://example.com/missing",
          "status_code": 404,
          "error": "Not Found"
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
      "discovered_at": "2026-11-04T12:00:00Z"
    }
  ]
}
```

## Report format

The top-level object always contains `root_url`, `depth`, `generated_at` and
`pages`. Every page contains `url`, `depth`, `http_status`, `status`, `seo`,
`broken_links`, `assets` and `discovered_at`.

- `status` is `ok` when the request succeeds with an HTTP status below 400;
  otherwise it is `error`.
- `http_status` is `0` when no response was received.
- `pages` is sorted by `depth`, then by `url`.
- `generated_at` and `discovered_at` are UTC timestamps in RFC 3339 format with
  second precision.
- `broken_links` contains only navigation links that returned HTTP 4xx/5xx or
  failed with a network error.
- `assets` contains images (`img[src]`), scripts (`script[src]`) and
  stylesheets (`link[rel~="stylesheet"]`), ordered images, scripts, styles.
- `size_bytes` comes from `Content-Length` when present, otherwise from the
  number of bytes actually read.
- Empty or zero fields (`error`, `status_code`, `size_bytes`) are omitted.
- On a network or body read error the page has `http_status: 0`, zero-value
  `seo`, and `broken_links`/`assets` set to `null`.

Links and assets are checked with `HEAD`, falling back to `GET` on `405`; assets
also fall back to `GET` when `HEAD` reports an unknown `Content-Length`. Pages
are fetched with `GET`.

## Development

| Command | Description |
| --- | --- |
| `make build` | build `bin/web-crawler` with the version from `git describe` |
| `make test` | run the test suite |
| `make test-race` | run tests with the race detector |
| `make cover` | write `coverage.out` and print a coverage summary |
| `make cover-show` | open the HTML coverage report |
| `make lint` | format the code and run `golangci-lint` |
| `make fmt` | format the code only |
| `make tidy` | run `go mod tidy` |
| `make clean` | remove build and coverage artifacts |
| `make snapshot` | build a local GoReleaser snapshot |

Before submitting changes run:

```sh
make test && make lint && make test-race
```

## Releasing

Releases are built by [GoReleaser](https://goreleaser.com) in the
`.github/workflows/release.yml` workflow when a `v*` tag is pushed. The workflow
produces archives for Linux, macOS and Windows (`amd64` and `arm64`), a
`checksums.txt` file and a GitHub release.

The version is injected at build time with
`-ldflags "-X main.version=<version>"`. `make build` takes the version from
`git describe`, while a plain `go build` reports `dev`.

Create and push a tag with the Makefile:

```sh
make tag VERSION=1.0.0
make tag-push VERSION=1.0.0
```

Or do both in one step:

```sh
make release VERSION=1.0.0
```

The `tag` target verifies that `VERSION` is a valid semver value, that the
working tree is clean and that the tag does not exist yet.

To validate the release configuration locally, install GoReleaser and run:

```sh
goreleaser check
make snapshot
```

## License

MIT. See [LICENSE](LICENSE).
