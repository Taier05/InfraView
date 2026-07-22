FROM node:22-alpine AS web-build

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run test:run
RUN npm run typecheck
RUN npm run build

FROM golang:1.24-bookworm AS go-build

WORKDIR /src
COPY go.mod ./
COPY docker-compose.yml ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=web-build /src/web/dist/ ./internal/httpapi/webdist/
RUN go test ./...
RUN go test -race ./...
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/infraview ./cmd/infraview

FROM alpine:3.21.3 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S -g 10001 infraview \
	&& adduser -S -D -H -u 10001 -G infraview infraview

COPY --from=go-build /out/infraview /usr/local/bin/infraview

USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=6 CMD ["/usr/local/bin/infraview", "healthcheck"]

ENTRYPOINT ["/usr/local/bin/infraview"]
CMD ["serve"]
