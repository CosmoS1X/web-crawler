BINARY := bin/web-crawler
COVERAGE_PROFILE := coverage.out
VERSION ?= $(shell git describe --tags --always --dirty --match 'v[0-9]*' 2>/dev/null || echo dev)
TAG ?= v$(VERSION)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build test test-race cover cover-show run fmt lint tidy clean tag tag-push release snapshot

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/web-crawler

fmt:
	golangci-lint fmt

lint: fmt
	golangci-lint run

test:
	go test ./...

test-race:
	go test -race ./...

cover:
	go test ./... -coverprofile=$(COVERAGE_PROFILE) -covermode=count
	go tool cover -func=$(COVERAGE_PROFILE)

cover-show: cover
	go tool cover -html=$(COVERAGE_PROFILE)

run:
	@test -n "$(URL)" || { echo "usage: make run URL=<url>" >&2; exit 1; }
	go run -ldflags "$(LDFLAGS)" ./cmd/web-crawler "$(URL)"

tidy:
	go mod tidy

clean:
	rm -rf bin dist tmp coverage.out

tag:
	@echo "$(VERSION)" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$$' \
		|| { echo "usage: make tag VERSION=1.2.3" >&2; exit 1; }
	@test -z "$$(git status --porcelain)" || { echo "working tree is not clean" >&2; exit 1; }
	@test -z "$$(git tag -l '$(TAG)')" || { echo "tag $(TAG) already exists" >&2; exit 1; }
	git tag -a "$(TAG)" -m "release $(TAG)"

tag-push:
	@echo "$(TAG)" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$$' \
		|| { echo "usage: make tag-push VERSION=1.2.3" >&2; exit 1; }
	git push origin "$(TAG)"

release: tag tag-push

snapshot:
	goreleaser release --snapshot --clean
