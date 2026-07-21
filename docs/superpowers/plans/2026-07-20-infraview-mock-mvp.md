# InfraView Mock MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Docker Compose deployable, fixed-login, read-only InfraView MVP that displays an infrastructure overview, a searchable Linux host list, and host details from a deterministic Mock data source.

**Architecture:** A Go HTTP service owns immutable configuration, authentication, in-memory sessions, read-only APIs, cache, aggregation, and data-source adapters. A React/TypeScript SPA calls only same-origin InfraView APIs; Vite builds the SPA and the production Go service serves its static assets. Production runs as one non-root InfraView container with no database and no writable business-data volume.

**Tech Stack:** Go 1.24, standard `net/http` and `log/slog`, React 19, TypeScript 5, Vite 7, React Router DOM 7, TanStack Query 5, TanStack Table 8, Apache ECharts 6, Vitest, Testing Library, Playwright, Docker 26+, Docker Compose v5.

## Global Constraints

- All product copy is Simplified Chinese except `InfraView`, CPU, I/O, IP, HTTP and established technical abbreviations.
- Do not add server, service, process, configuration, alert, restart, delete, task, shell, SSH, remote command execution, or generic upstream proxy operations.
- Authentication uses one username and password from environment variables; there is no user-management UI or API.
- The browser never receives upstream credentials or arbitrary upstream query capability.
- Production is one application container, with no database and no persistent monitoring-data volume.
- Default refresh is 30 seconds; supported ranges are 1 hour, 6 hours, 24 hours, and 7 days.
- Default cache TTLs are inventory 60 seconds, current metrics 20 seconds, range data 60 seconds, and data-source health 15 seconds.
- Stale data may be served for at most 5 minutes and must show its collection time.
- Default warning and critical thresholds are 80% and 90%, configurable only at startup.
- Target capacity is 100 Linux hosts, cached API P95 below 200 ms, and idle container memory below 256 MiB.
- Follow TDD for every application change: failing test, confirmed failure, minimum implementation, confirmed pass, commit.
- The host has no Go installation. Use `golang:1.24-bookworm` containers for Go formatting, tests, race tests, and builds; do not install Go on the host.
- Keep `docs/PROJECT_STATUS.md` and `docs/TODO.md` current after each completed milestone.

---

## File Map

- `cmd/infraview/main.go`: startup, dependency wiring, graceful shutdown, healthcheck command.
- `internal/config/`: immutable environment configuration and validation.
- `internal/auth/`: fixed credentials, memory sessions, login limiter.
- `internal/cache/`: TTL entries, stale fallback, request coalescing.
- `internal/datasource/`: normalized domain types, provider interface, contract tests.
- `internal/adapters/mock/`: deterministic development and test data.
- `internal/adapters/nightingale/`: compile-safe unconfigured provider without guessed API calls.
- `internal/service/`: overview, hosts, detail, history, thresholds, source status.
- `internal/httpapi/`: routes, middleware, JSON envelopes, static assets.
- `web/src/api/`: same-origin client and types matching Go JSON.
- `web/src/auth/`: session bootstrap, login, logout.
- `web/src/components/`: shared cards, status, range, chart, stale and error components.
- `web/src/features/overview/`: overview page.
- `web/src/features/hosts/`: host list and detail pages.
- `web/e2e/`: Playwright critical path.
- `Dockerfile`, `docker-compose.yml`, `.env.example`, `scripts/`: delivery and verification.
- `docs/`: architecture, design, configuration, deployment, development, testing, security, decisions, status, TODO, and Nightingale notes.

---

### Task 1: Bootstrap Go Configuration, Health Check, and Dockerized Developer Commands

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/httpapi/api.go`
- Create: `internal/httpapi/api_test.go`
- Create: `cmd/infraview/main.go`

**Interfaces:**
- Produces: `config.Load(getenv func(string) string) (config.Config, error)`.
- Produces: `httpapi.New(httpapi.Dependencies) http.Handler` with `GET /healthz`.
- Produces: `infraview serve` and `infraview healthcheck`.

- [ ] **Step 1: Create the module and Go command wrappers**

Create `go.mod`:

```go
module github.com/Taier05/InfraView

go 1.24
```

Create `Makefile` targets `gofmt`, `go-test`, `go-test-race`, and `go-build` using this container prefix:

```make
GO_IMAGE ?= golang:1.24-bookworm
GO_DOCKER = docker run --rm --user "$$(id -u):$$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$$(pwd)":/src -w /src $(GO_IMAGE)
```

The `gofmt` target must enumerate files with `find cmd internal -type f -name '*.go'` before passing them to `gofmt -l`. The `go-build` target must write `/tmp/infraview` inside the container so verification does not create an untracked binary in the repository.

- [ ] **Step 2: Write failing configuration tests**

```go
func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(mapEnv(map[string]string{
		"INFRAVIEW_USERNAME": "admin",
		"INFRAVIEW_PASSWORD": "secret-value",
	}))
	if err != nil { t.Fatal(err) }
	if cfg.ListenAddr != ":8080" { t.Fatalf("listen = %q", cfg.ListenAddr) }
	if cfg.DataSource != "mock" { t.Fatalf("source = %q", cfg.DataSource) }
	if cfg.SessionTTL != 12*time.Hour { t.Fatalf("session = %s", cfg.SessionTTL) }
	if cfg.WarningPercent != 80 || cfg.CriticalPercent != 90 { t.Fatalf("thresholds = %v/%v", cfg.WarningPercent, cfg.CriticalPercent) }
}
```

Also test missing credentials, password shorter than 12 characters, invalid duration, unsupported source, Mock host count outside `1..100`, invalid percentage, and warning not lower than critical.

- [ ] **Step 3: Prove configuration tests fail**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD":/src -w /src golang:1.24-bookworm go test ./internal/config -v
```

Expected: FAIL because `Config` and `Load` do not exist.

- [ ] **Step 4: Implement exact configuration fields**

```go
type Config struct {
	ListenAddr string
	Username string
	Password string
	SessionTTL time.Duration
	CookieSecure bool
	DataSource string
	MockHostCount int
	RefreshInterval time.Duration
	InventoryTTL time.Duration
	CurrentMetricsTTL time.Duration
	RangeTTL time.Duration
	HealthTTL time.Duration
	MaxStale time.Duration
	UpstreamTimeout time.Duration
	WarningPercent float64
	CriticalPercent float64
}
```

Use environment defaults `:8080`, `12h`, `false`, `mock`, `32`, `30s`, `60s`, `20s`, `60s`, `15s`, `5m`, `10s`, `80`, `90` in field order. Return descriptive errors for every tested invalid value.

- [ ] **Step 5: Write and prove a failing health test**

```go
func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	New(Dependencies{}).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK { t.Fatalf("status = %d", rec.Code) }
	if strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` { t.Fatalf("body = %q", rec.Body.String()) }
}
```

Run `make go-test`. Expected: FAIL because the API constructor does not exist.

- [ ] **Step 6: Implement API startup and healthcheck command**

Register only `GET /healthz` initially. Build `cmd/infraview/main.go` to load configuration, create an `http.Server`, handle SIGINT/SIGTERM, shut down within 10 seconds, and exit non-zero on startup errors. The `healthcheck` command parses `INFRAVIEW_LISTEN_ADDR`, replaces wildcard hosts with `127.0.0.1`, and calls the resulting `/healthz` URL with a 2-second timeout.

- [ ] **Step 7: Verify and commit**

```bash
make gofmt go-test go-test-race go-build
git add go.mod Makefile cmd internal/config internal/httpapi
git commit -m "feat: bootstrap InfraView service"
```

Expected: all checks exit 0 before the commit.

### Task 2: Define Normalized Data Types and Deterministic Mock Provider

**Files:**
- Create: `internal/datasource/types.go`
- Create: `internal/datasource/provider.go`
- Create: `internal/datasource/contract_test.go`
- Create: `internal/adapters/mock/provider.go`
- Create: `internal/adapters/mock/provider_test.go`
- Create: `internal/adapters/nightingale/provider.go`

**Interfaces:**
- Produces: `datasource.Provider`.
- Produces: `mock.New(hostCount int, clock func() time.Time) datasource.Provider`.

- [ ] **Step 1: Write the shared provider contract first**

```go
type Provider interface {
	Health(context.Context) (Health, error)
	ListHosts(context.Context) ([]Host, error)
	GetHost(context.Context, string) (Host, error)
	GetCurrentMetrics(context.Context, []string) (map[string]CurrentMetrics, error)
	QueryRange(context.Context, RangeRequest) ([]Series, error)
}
```

`RunContract` checks non-empty inventory, unique stable IDs, one-host lookup, batch current metrics, 61 points for a one-hour query at one-minute steps, valid timestamps, and `ErrNotFound` for an unknown host.

- [ ] **Step 2: Prove provider tests fail**

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD":/src -w /src golang:1.24-bookworm go test ./internal/datasource ./internal/adapters/mock -v
```

Expected: FAIL because provider types do not exist.

- [ ] **Step 3: Define normalized types**

Define statuses `online`, `offline`, `unknown`; metric keys for CPU, memory, load 1, disk usage, disk read/write bytes per second, and network receive/transmit bytes per second. `Host` contains ID, name, IP, OS, status, status time, uptime. `CurrentMetrics` uses `*float64` fields and contains filesystems. `Point` has a nullable value. Define `Health`, `Series`, `RangeRequest`, and errors `ErrNotFound`, `ErrUnavailable`, `ErrNotConfigured`.

- [ ] **Step 4: Implement deterministic Mock behavior**

Generate hosts `mock-host-001` through the configured count, names `linux-001`, IPs in `192.0.2.0/24`, and stable Linux metadata. Every seventeenth host is offline. Generate values from host index and timestamp with bounded arithmetic and `math.Sin`; never use random numbers. Identical calls with a fixed clock must be deeply equal.

The Nightingale provider compiles and returns `ErrNotConfigured` for every method; do not add upstream endpoint guesses.

- [ ] **Step 5: Verify and commit**

```bash
make gofmt go-test go-test-race
git add internal/datasource internal/adapters
git commit -m "feat: add data source contract and mock provider"
```

Expected: the shared contract passes against Mock and race tests are clean.

### Task 3: Add TTL Cache, Stale Fallback, and Read-Only Query Services

**Files:**
- Create: `internal/cache/store.go`
- Create: `internal/cache/store_test.go`
- Create: `internal/service/service.go`
- Create: `internal/service/types.go`
- Create: `internal/service/overview.go`
- Create: `internal/service/hosts.go`
- Create: `internal/service/metrics.go`
- Create: `internal/service/status.go`
- Create: `internal/service/service_test.go`

**Interfaces:**
- Produces: `cache.Store.GetOrLoad(ctx, key, ttl, maxStale, loader)`.
- Produces: `Service.Overview`, `Hosts`, `Host`, `Metrics`, `DataSourceStatus`.

```go
type Loader func(context.Context) (any, error)
func (s *Store) GetOrLoad(ctx context.Context, key string, ttl, maxStale time.Duration, loader Loader) (Result, error)
```

- [ ] **Step 1: Write failing cache lifecycle and concurrency tests**

Test fresh hit, expiry, stale return after loader failure, hard failure after 5 minutes, and 20 concurrent misses invoking the loader once. Use an injected clock; do not sleep.

```go
type State string
const (
	Fresh State = "fresh"
	Stale State = "stale"
)
type Result struct { Value any; State State; StoredAt time.Time }
```

- [ ] **Step 2: Prove cache tests fail**

Run:

```bash
docker run --rm --user "$(id -u):$(id -g)" -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/tmp/go-mod -v "$PWD":/src -w /src golang:1.24-bookworm go test ./internal/cache -v
```

Expected: FAIL because `Store` does not exist.

- [ ] **Step 3: Implement cache without holding locks during loaders**

Use a mutex-protected entry map and a separate in-flight map with a `done` channel per key. Return stale only when the loader fails and the stored value is not older than `maxStale`. Context cancellation stops a waiting caller without cancelling a loader still needed by other callers.

- [ ] **Step 4: Write failing service tests**

Test:

- overview total/online/offline counts and non-null CPU/memory averages;
- search by name/IP, status filter, stable sort, page size `1..100`;
- host detail, filesystems, thresholds, and not-found mapping;
- ranges `1h`, `6h`, `24h`, `7d` and at most 600 points;
- one batch current-metrics call for overview and host list;
- null metrics excluded from averages and marked unknown;
- stale metadata propagated from cache;
- source health cached for 15 seconds.

- [ ] **Step 5: Prove service tests fail**

Run Dockerized `go test ./internal/service -v`. Expected: FAIL because service methods do not exist.

- [ ] **Step 6: Implement exact service methods**

```go
func New(provider datasource.Provider, store *cache.Store, options Options) *Service
func (s *Service) Overview(ctx context.Context, rangeName string) (Overview, Meta, error)
func (s *Service) Hosts(ctx context.Context, query HostQuery) (HostPage, Meta, error)
func (s *Service) Host(ctx context.Context, id string) (HostDetail, Meta, error)
func (s *Service) Metrics(ctx context.Context, id, rangeName string) (MetricRange, Meta, error)
func (s *Service) DataSourceStatus(ctx context.Context) (DataSourceStatus, Meta, error)
```

Use levels `normal`, `warning`, `critical`, `unknown`. Only CPU, memory, and disk percentages use configured thresholds. Online state comes from the provider, never from a metric value of zero.

Define these shared service inputs exactly:

```go
type Options struct {
	InventoryTTL time.Duration
	CurrentMetricsTTL time.Duration
	RangeTTL time.Duration
	HealthTTL time.Duration
	MaxStale time.Duration
	WarningPercent float64
	CriticalPercent float64
	Clock func() time.Time
}
type Meta struct { Stale bool; CollectedAt time.Time }
type HostQuery struct {
	Search string
	Status datasource.HostStatus
	Sort string
	Order string
	Page int
	PageSize int
}
```

`Overview`, `HostPage`, `HostDetail`, `MetricRange`, and `DataSourceStatus` must use only normalized datasource fields plus server-computed levels and pagination metadata; none may expose provider-specific raw payloads.

- [ ] **Step 7: Verify and commit**

```bash
make gofmt go-test go-test-race
git add internal/cache internal/service
git commit -m "feat: add cached infrastructure query services"
```

Expected: all service and cache tests pass under the race detector.

### Task 4: Implement Fixed Login, Sessions, Rate Limit, and Read-Only API

**Files:**
- Create: `internal/auth/manager.go`
- Create: `internal/auth/manager_test.go`
- Create: `internal/auth/limiter.go`
- Create: `internal/auth/limiter_test.go`
- Modify: `internal/httpapi/api.go`
- Create: `internal/httpapi/auth_handlers.go`
- Create: `internal/httpapi/query_handlers.go`
- Create: `internal/httpapi/middleware.go`
- Create: `internal/httpapi/respond.go`
- Modify: `internal/httpapi/api_test.go`
- Modify: `cmd/infraview/main.go`

**Interfaces:**
- Produces: `Manager.Login`, `Validate`, `Logout` and a per-IP limiter.
- Produces: all `/api/v1` routes from the design specification.

- [ ] **Step 1: Write failing authentication primitive tests**

```go
type Session struct { Token string; ExpiresAt time.Time }
func NewManager(username, password string, ttl time.Duration, random io.Reader, clock func() time.Time) *Manager
func (m *Manager) Login(username, password string) (Session, bool)
func (m *Manager) Validate(token string) bool
func (m *Manager) Logout(token string)
```

Test valid and invalid credentials, unique URL-safe 32-byte tokens, 12-hour expiry, logout, concurrent validation, five failures per IP per minute, sixth blocked, success reset, and next-window recovery.

Define the limiter API exactly:

```go
func NewLimiter(limit int, window time.Duration, clock func() time.Time) *Limiter
func (l *Limiter) Allow(ip string) bool
func (l *Limiter) RecordFailure(ip string)
func (l *Limiter) Reset(ip string)
```

- [ ] **Step 2: Prove auth tests fail, then implement**

Run Dockerized `go test ./internal/auth -v`. Expected: FAIL. Implement SHA-256 plus `subtle.ConstantTimeCompare` for submitted/configured credentials, store only token hashes, prune expiry during login/validation, and protect maps with mutexes.

- [ ] **Step 3: Write failing HTTP integration tests**

Cover:

```text
POST   /api/v1/session
DELETE /api/v1/session
GET    /api/v1/session
GET    /api/v1/overview?range=24h
GET    /api/v1/hosts?q=linux&status=online&sort=cpu&order=desc&page=1&page_size=20
GET    /api/v1/hosts/mock-host-001
GET    /api/v1/hosts/mock-host-001/metrics?range=6h
GET    /api/v1/datasource/status
GET    /healthz
```

Assert 401 before login, 204 plus `HttpOnly`/`SameSite=Strict` cookie after login, 429 on the sixth failure, 400 for invalid range/page/sort, 404 for unknown host, and 503 for unavailable source without stale data.

- [ ] **Step 4: Implement envelopes, middleware, and route allowlist**

```go
type ResponseMeta struct {
	RequestID string `json:"request_id"`
	Stale bool `json:"stale"`
	CollectedAt time.Time `json:"collected_at,omitempty"`
}
type ErrorBody struct {
	Code string `json:"code"`
	Message string `json:"message"`
	RequestID string `json:"request_id"`
	Retryable bool `json:"retryable"`
}
```

Add request ID, recovery, security headers, structured logs, same-origin validation for login/logout, and authentication. Register only approved routes. Set CSP, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and API `Cache-Control: no-store`. Decode login with unknown-field rejection and a 4 KiB body limit. Never serialize raw upstream errors.

Task 4 changes `httpapi.Dependencies` to:

```go
type Dependencies struct {
	Config config.Config
	Auth *auth.Manager
	Limiter *auth.Limiter
	Service *service.Service
	Logger *slog.Logger
}
```

- [ ] **Step 5: Wire dependencies and verify**

Construct Mock or compile-safe Nightingale provider from config, then cache, service, auth, limiter, logger, and API. Wrap provider calls with configured upstream timeout.

```bash
make gofmt go-test go-test-race go-build
git add cmd/infraview internal/auth internal/httpapi
git commit -m "feat: expose authenticated read-only API"
```

Expected: all API/auth tests pass and there are no business write routes.

### Task 5: Bootstrap React, Dark Chinese Theme, API Client, and Login

**Files:**
- Create: `web/package.json`
- Create: `web/package-lock.json`
- Create: `web/tsconfig.json`
- Create: `web/vite.config.ts`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/app/App.tsx`
- Create: `web/src/app/AppShell.tsx`
- Create: `web/src/app/theme.css`
- Create: `web/src/api/client.ts`
- Create: `web/src/api/types.ts`
- Create: `web/src/auth/AuthProvider.tsx`
- Create: `web/src/auth/LoginPage.tsx`
- Create: `web/src/auth/LoginPage.test.tsx`
- Create: `web/src/test/setup.ts`
- Create: `web/src/test/server.ts`
- Create: `web/src/test/fixtures.ts`

**Interfaces:**
- Produces: `apiRequest<T>`, `AuthProvider`, `useAuth`, authenticated routing, and the approved dark shell.

```ts
export async function apiRequest<T>(path: string, init?: RequestInit): Promise<T>
```

- [ ] **Step 1: Create package scripts and install the lockfile**

Use runtime versions React `19.1.0`, React DOM `19.1.0`, React Router DOM `7.18.1`, TanStack Query `5.80.7`, TanStack Table `8.21.3`, and ECharts `6.1.0`. Use development versions TypeScript `5.8.3`, Vite `7.3.6`, Vitest `3.2.7`, jsdom `26.1.0`, Testing Library React `16.3.0`, user-event `14.6.1`, MSW `2.10.2`, and Playwright `1.61.1`. Set Node engine `>=22` and scripts `dev`, `build`, `test`, `test:run`, `typecheck`, `e2e`.

These versions supersede the original pins after `npm audit --omit=dev` found production vulnerabilities in React Router DOM `7.6.2` and ECharts `5.6.0`. Do not downgrade to the superseded versions.

Run:

```bash
cd web
npm install
```

Expected: `package-lock.json` is created without install errors.

- [ ] **Step 2: Configure Vite and write failing login tests**

Proxy `/api` and `/healthz` to `http://127.0.0.1:8080` in development. Configure jsdom and `src/test/setup.ts`.

```tsx
it('登录后进入基础设施总览', async () => {
  render(<App />)
  await user.type(screen.getByLabelText('用户名'), 'admin')
  await user.type(screen.getByLabelText('密码'), 'secret-value')
  await user.click(screen.getByRole('button', { name: '登录' }))
  expect(await screen.findByRole('heading', { name: '基础设施总览' })).toBeInTheDocument()
})
```

Also test invalid credentials, disabled submit while pending, cleared password after failure, and redirect to `/login`.

- [ ] **Step 3: Prove login tests fail**

```bash
cd web
npm run test:run -- src/auth/LoginPage.test.tsx
```

Expected: FAIL because the application and login components do not exist.

- [ ] **Step 4: Implement client, auth state, and shell**

`APIError` contains status, code, Chinese message, request ID, retryable. Fetch uses same-origin credentials and never renders raw HTML. `AuthProvider` bootstraps `GET /api/v1/session`, exposes login/logout, and clears queries on logout.

Use approved low-saturation dark CSS variables, visible keyboard focus, skip link, Chinese navigation `总览` and `主机`, source-status area, and only refresh/range/logout controls.

- [ ] **Step 5: Verify and commit**

```bash
cd web
npm run test:run
npm run typecheck
npm run build
cd ..
git add web
git commit -m "feat: add dark Chinese web shell and login"
```

Expected: tests, typecheck, and build pass; `web/dist/index.html` exists.

### Task 6: Implement Infrastructure Overview

**Files:**
- Create: `web/src/components/MetricCard.tsx`
- Create: `web/src/components/StatusBadge.tsx`
- Create: `web/src/components/TimeRangeSelector.tsx`
- Create: `web/src/components/TrendChart.tsx`
- Create: `web/src/components/StaleBanner.tsx`
- Create: `web/src/components/ErrorPanel.tsx`
- Create: `web/src/features/overview/OverviewPage.tsx`
- Create: `web/src/features/overview/OverviewPage.test.tsx`
- Modify: `web/src/app/App.tsx`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`

**Interfaces:**
- Consumes: `GET /api/v1/overview?range={1h|6h|24h|7d}`.
- Produces: reusable metric, status, range, chart, stale, and error components.

- [ ] **Step 1: Write failing overview tests**

Assert four primary cards, default `24小时`, all four ranges, 30-second refetch without overlap, stale banner with timestamp, `暂无数据` for null values, retryable Chinese error panel, and text labels in addition to colors.

- [ ] **Step 2: Prove overview tests fail**

```bash
cd web
npm run test:run -- src/features/overview/OverviewPage.test.tsx
```

Expected: FAIL because overview components do not exist.

- [ ] **Step 3: Implement accessible shared components**

`TimeRangeSelector` uses buttons with `aria-pressed`. `TrendChart` imports only line, grid, tooltip, time axis, and canvas modules from ECharts and includes a screen-reader summary. `ErrorPanel` shows `重试` only for retryable errors. `StaleBanner` displays exact collection time.

- [ ] **Step 4: Implement overview query behavior**

Use query key `['overview', range]`, `refetchInterval: 30000`, `refetchIntervalInBackground: false`, and cancellation on range change. Render server-provided levels instead of recalculating authoritative thresholds in the browser.

- [ ] **Step 5: Verify and commit**

```bash
cd web
npm run test:run
npm run typecheck
npm run build
cd ..
git add web/src
git commit -m "feat: add infrastructure overview"
```

Expected: overview and all earlier frontend checks pass.

### Task 7: Implement Host Search, Filter, Sort, and Pagination

**Files:**
- Create: `web/src/features/hosts/HostListPage.tsx`
- Create: `web/src/features/hosts/HostListPage.test.tsx`
- Modify: `web/src/app/App.tsx`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`

**Interfaces:**
- Consumes: `/api/v1/hosts` with `q`, `status`, `sort`, `order`, `page`, `page_size`.
- Produces: URL-backed list state and links to `/hosts/:id`.

- [ ] **Step 1: Write failing host-list tests**

Assert label `搜索主机名或 IP`, 300 ms debounce, status values `全部状态/在线/离线`, sortable host/CPU/memory/load/uptime columns, fixed page size 20, URL-preserved state, row link to detail, null metric text, and absence of `重启/删除/执行/修改` controls.

- [ ] **Step 2: Prove host-list tests fail**

```bash
cd web
npm run test:run -- src/features/hosts/HostListPage.test.tsx
```

Expected: FAIL because `HostListPage` does not exist.

- [ ] **Step 3: Implement server-driven table state**

Use TanStack Table for rendering headers and rows only. The API owns sorting and pagination. Include every URL parameter in the query key and reset page to 1 after search, filter, or sort changes. Preserve URL state when returning from a detail page. Format percentages to one decimal and uptime to Chinese day/hour text.

- [ ] **Step 4: Verify and commit**

```bash
cd web
npm run test:run
npm run typecheck
npm run build
cd ..
git add web/src
git commit -m "feat: add searchable host list"
```

Expected: host-list and earlier tests pass.

### Task 8: Implement Host Detail and Historical Metrics

**Files:**
- Create: `web/src/features/hosts/HostDetailPage.tsx`
- Create: `web/src/features/hosts/HostDetailPage.test.tsx`
- Modify: `web/src/app/App.tsx`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/test/fixtures.ts`

**Interfaces:**
- Consumes: `GET /api/v1/hosts/{id}` and `GET /api/v1/hosts/{id}/metrics?range=...`.
- Produces: `/hosts/:id` current metrics and historical charts.

- [ ] **Step 1: Write failing host-detail tests**

Assert identity, IP, OS, status, uptime, CPU, memory, load, filesystems, disk read/write, network receive/transmit, four time ranges, stale/error handling, per-card null values, and missing-host message `该主机当前不在数据源中`.

- [ ] **Step 2: Prove host-detail tests fail**

```bash
cd web
npm run test:run -- src/features/hosts/HostDetailPage.test.tsx
```

Expected: FAIL because `HostDetailPage` does not exist.

- [ ] **Step 3: Implement independent current and range queries**

Use keys `['host', id]` and `['host-metrics', id, range]`; refresh current data every 30 seconds and history every 60 seconds. One query failure must not hide successful data from the other. Cancel on ID/range change and unmount.

- [ ] **Step 4: Implement the approved detail layout**

Order: metadata/current cards, CPU-memory-load trend, filesystem capacity table, disk I/O chart, network chart. Do not add process, service, command, configuration, alert, or operation controls.

- [ ] **Step 5: Verify and commit**

```bash
cd web
npm run test:run
npm run typecheck
npm run build
cd ..
git add web/src
git commit -m "feat: add host detail metrics"
```

Expected: host-detail and all prior frontend tests pass.

### Task 9: Serve the SPA from Go and Add Secure Docker Compose Delivery

**Files:**
- Create: `internal/httpapi/web.go`
- Create: `internal/httpapi/web_test.go`
- Modify: `internal/httpapi/api.go`
- Modify: `internal/httpapi/middleware.go`
- Modify: `Makefile`
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `.env.example`
- Create: `scripts/smoke.sh`
- Modify: `.gitignore`

**Interfaces:**
- Produces: one HTTP origin serving SPA and API.
- Produces: `docker compose up -d --build` deployment.

- [ ] **Step 1: Write failing static and route-surface tests**

Assert `/` and client routes return HTML, fingerprinted assets use immutable cache, index uses no-cache, `/api/v1/not-real` stays JSON 404, dotfiles/path traversal are rejected, and command/restart/delete/proxy/query routes return 404 or 405. Capture logs and assert known password, cookie, and token strings are absent.

- [ ] **Step 2: Prove web tests fail**

Run `make go-test`. Expected: FAIL because static assets are not registered.

- [ ] **Step 3: Build and serve frontend assets**

Add `make web-copy` to recreate only ignored directory `internal/httpapi/webdist` from `web/dist`, then use `go:embed`. From this task onward, `go-test`, `go-test-race`, and `go-build` depend on `web-copy`, while `web-copy` depends on a successful frontend build. Never SPA-fallback `/api/`. Serve hashed assets for one year and `index.html` with no-cache.

- [ ] **Step 4: Write smoke test before container files**

`scripts/smoke.sh` uses `set -eu`, a `mktemp` cookie file with trap cleanup, 60-second health polling, login, every allowed read API, root HTML, and disallowed-route assertions. It prints `smoke: PASS` only after success.

Run:

```bash
INFRAVIEW_BASE_URL=http://127.0.0.1:8080 INFRAVIEW_USERNAME=admin INFRAVIEW_PASSWORD=change-me-please ./scripts/smoke.sh
```

Expected: FAIL because no deployment exists.

- [ ] **Step 5: Create multi-stage Docker and Compose**

Use `node:22-alpine` for npm tests/typecheck/build, `golang:1.24-bookworm` for Go tests/race tests/build, and `alpine:3.21.3` for the runtime with CA certificates and timezone data only. Run as UID/GID 10001.

Compose contains exactly one service, `${INFRAVIEW_PORT:-8080}:8080`, `.env`, `read_only: true`, `/tmp` tmpfs, all capabilities dropped, no-new-privileges, user 10001, and binary healthcheck. Do not mount Docker Socket or writable business volumes.

- [ ] **Step 6: Verify deployment and memory**

```bash
test ! -e .env
cp .env.example .env
docker compose up -d --build
docker compose ps
INFRAVIEW_BASE_URL=http://127.0.0.1:8080 INFRAVIEW_USERNAME=admin INFRAVIEW_PASSWORD=change-me-please ./scripts/smoke.sh
docker stats --no-stream --format '{{.Name}} {{.MemUsage}}' "$(docker compose ps -q infraview)"
docker compose down
```

Expected: service healthy, smoke passes, idle memory below 256 MiB, and only project Compose resources are removed.

- [ ] **Step 7: Commit delivery**

```bash
git add Dockerfile docker-compose.yml .env.example scripts/smoke.sh Makefile .gitignore cmd internal/httpapi
git commit -m "feat: add secure single-container deployment"
```

### Task 10: Add E2E, Cached Latency Check, Durable Documentation, and Final Verification

**Files:**
- Create: `web/playwright.config.ts`
- Create: `web/e2e/infraview.spec.ts`
- Create: `scripts/benchmark.sh`
- Create: `docs/ARCHITECTURE.md`
- Create: `docs/DESIGN.md`
- Create: `docs/CONFIGURATION.md`
- Create: `docs/DEPLOYMENT.md`
- Create: `docs/DEVELOPMENT.md`
- Create: `docs/TESTING.md`
- Create: `docs/SECURITY.md`
- Create: `docs/datasources/NIGHTINGALE.md`
- Create: `docs/decisions/0001-single-container-go-react.md`
- Modify: `README.md`
- Modify: `docs/PROJECT_STATUS.md`
- Modify: `docs/TODO.md`
- Modify: `Makefile`

**Interfaces:**
- Produces: `make verify`, browser acceptance, latency evidence, and repository context recovery.

- [ ] **Step 1: Write failing Playwright critical path**

Cover login, overview cards, 7-day selection, host search/filter, detail metrics, refresh, controlled stale/error states, logout, login redirect, and absence of destructive controls.

Run:

```bash
cd web
npx playwright install chromium
npm run e2e
```

Expected: FAIL because browser orchestration is not configured.

- [ ] **Step 2: Configure isolated E2E**

Use Chromium, base URL `http://127.0.0.1:18080`, trace on first retry, and a dedicated Compose project/port. Teardown always runs and never affects other Compose projects.

- [ ] **Step 3: Add cached latency acceptance**

`scripts/benchmark.sh` logs in, warms overview, performs 100 sequential authenticated requests, sorts curl `time_total`, selects sample 95, and fails at `>=0.200` seconds. Print `benchmark: PASS p95=<value>s` on success.

- [ ] **Step 4: Complete durable documentation**

Document architecture, directories, every environment variable, direct IP/port access, optional Nginx and Caddy examples, upgrades/rollback, health checks, test commands, security/read-only boundaries, resource checks, no-data-backup implications, and Nightingale evidence required before integration. Update project status with exact commits and verification evidence; mark TODO items only after they pass.

- [ ] **Step 5: Add and run full verification**

`make verify` executes frontend unit tests, typecheck, build, asset copy, Go formatting, Go tests, race tests, Go build, image build, Compose smoke, Playwright, latency check, and teardown.

```bash
make verify
git diff --check
git status --short
```

Expected: all commands exit 0; whitespace check is silent; status shows only Task 10 changes.

- [ ] **Step 6: Commit final MVP evidence**

```bash
git add README.md Makefile web/playwright.config.ts web/e2e scripts/benchmark.sh docs
git commit -m "test: verify and document InfraView MVP"
git status --short --branch
git log --oneline --decorate -15
```

Expected: clean working tree and focused commits for Tasks 1-10. Do not push or merge unless the user explicitly requests it.
