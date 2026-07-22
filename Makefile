GO_IMAGE ?= golang:1.24-bookworm
GO_DOCKER = docker run --rm --user "$$(id -u):$$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$$(pwd)":/src -w /src $(GO_IMAGE)
NODE_IMAGE ?= node:22-alpine
NODE_DOCKER = docker run --rm --user "$$(id -u):$$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$$(pwd)":/src -w /src/web $(NODE_IMAGE)

.NOTPARALLEL: verify

.PHONY: gofmt go-test go-test-race go-build web-test web-typecheck web-audit web-build web-copy image-build e2e-safety-test acceptance verify

gofmt:
	@files="$$(find cmd internal -type f -name '*.go')"; \
	unformatted="$$($(GO_DOCKER) gofmt -l $$files)" || exit $$?; \
	test -z "$$unformatted"

web-build:
	$(NODE_DOCKER) sh -c 'npm ci && npm run build'

web-test:
	$(NODE_DOCKER) sh -c 'npm ci && npm run test:run'

web-typecheck:
	$(NODE_DOCKER) sh -c 'npm ci && npm run typecheck'

web-audit:
	$(NODE_DOCKER) sh -c 'npm ci && npm audit --omit=dev && npm audit'

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

image-build:
	docker build --tag infraview:verify .

e2e-safety-test:
	./scripts/e2e-safety.test.sh

acceptance:
	cd web && INFRAVIEW_E2E_PORT=18080 INFRAVIEW_E2E_RUN_BENCHMARK=true INFRAVIEW_E2E_CHECK_RESOURCES=true npm run e2e

verify: web-test web-typecheck web-audit web-copy gofmt go-test go-test-race go-build image-build e2e-safety-test acceptance
