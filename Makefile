GO_IMAGE ?= golang:1.24-bookworm
GO_DOCKER = docker run --rm --user "$$(id -u):$$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$$(pwd)":/src -w /src $(GO_IMAGE)
NODE_IMAGE ?= node:22-alpine
NODE_DOCKER = docker run --rm --user "$$(id -u):$$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$$(pwd)":/src -w /src/web $(NODE_IMAGE)

.PHONY: gofmt go-test go-test-race go-build web-build web-copy

gofmt:
	@files="$$(find cmd internal -type f -name '*.go')"; \
	unformatted="$$($(GO_DOCKER) gofmt -l $$files)" || exit $$?; \
	test -z "$$unformatted"

web-build:
	$(NODE_DOCKER) sh -c 'npm ci && npm run build'

web-copy: web-build
	git check-ignore -q internal/httpapi/webdist/
	test ! -L internal/httpapi/webdist
	mkdir -p internal/httpapi/webdist
	test "$$(realpath internal/httpapi/webdist)" = "$$(pwd)/internal/httpapi/webdist"
	find internal/httpapi/webdist -mindepth 1 -delete
	cp -R web/dist/. internal/httpapi/webdist/

go-test: web-copy
	$(GO_DOCKER) go test ./...

go-test-race: web-copy
	$(GO_DOCKER) go test -race ./...

go-build: web-copy
	$(GO_DOCKER) go build -o /tmp/infraview ./cmd/infraview
