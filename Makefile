GO_IMAGE ?= golang:1.24-bookworm
GO_DOCKER = docker run --rm --user "$$(id -u):$$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$$(pwd)":/src -w /src $(GO_IMAGE)

.PHONY: gofmt go-test go-test-race go-build

gofmt:
	@files="$$(find cmd internal -type f -name '*.go')"; test -z "$$($(GO_DOCKER) gofmt -l $$files)"

go-test:
	$(GO_DOCKER) go test ./...

go-test-race:
	$(GO_DOCKER) go test -race ./...

go-build:
	$(GO_DOCKER) go build -o /tmp/infraview ./cmd/infraview
