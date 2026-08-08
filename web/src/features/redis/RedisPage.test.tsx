import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";

import { RedisPage } from "./RedisPage";

const requests: URL[] = [];
let pageSize500Total = 1001;
const redisSortFields = [
  ["实例地址", "instance"],
  ["角色", "role"],
  ["内存上限", "memory_limit"],
  ["内存使用率", "memory"],
  ["连接", "connections"],
  ["阻塞连接", "blocked_connections"],
  ["QPS", "qps"],
  ["命中率", "hit_rate"],
  ["key 总数", "keys"],
  ["复制链路", "replication_link"],
  ["延迟", "replication_lag"],
  ["运行时间", "uptime"],
  ["状态", "status"],
] as const;
const fixture = {
  data: {
    instances: [
      {
        id: "fixture-a",
        address: "192.0.2.40:6379",
        availability: "up",
        role: "master",
        cluster_enabled: true,
        used_memory_bytes: 1073741824,
        max_memory_bytes: 2147483648,
        memory_usage_percent: 50,
        connected_clients: 12,
        max_clients: 100,
        connection_usage_percent: 12,
        blocked_clients: 1,
        qps: 25.5,
        hit_rate: 0.75,
        keys: 4096,
        expired_keys_per_second: 0.2,
        evicted_keys_per_second: 0,
        rejected_connections_rate: 0,
        replication: {
          connected_replicas: 1,
          master_link_up: null,
          master_last_io_seconds_ago: null,
          master_sync_in_progress: null,
          worst_replica_lag_seconds: 2,
        },
        uptime_seconds: 90,
        status: "normal",
        status_source: "normal",
        collection_level: "normal",
      },
      {
        id: "fixture-b",
        address: "192.0.2.41:6379",
        availability: "up",
        role: "slave",
        cluster_enabled: true,
        used_memory_bytes: null,
        max_memory_bytes: 0,
        memory_usage_percent: null,
        connected_clients: null,
        max_clients: null,
        connection_usage_percent: null,
        blocked_clients: null,
        qps: null,
        hit_rate: null,
        keys: null,
        expired_keys_per_second: null,
        evicted_keys_per_second: null,
        rejected_connections_rate: null,
        replication: {
          connected_replicas: null,
          master_link_up: true,
          master_last_io_seconds_ago: 1,
          master_sync_in_progress: true,
          worst_replica_lag_seconds: null,
        },
        uptime_seconds: null,
        status: "normal",
        status_source: "normal",
        collection_level: "normal",
      },
      {
        id: "fixture-c",
        address: "192.0.2.42:6379",
        availability: "up",
        role: "slave",
        cluster_enabled: true,
        used_memory_bytes: 536870912,
        max_memory_bytes: null,
        memory_usage_percent: null,
        connected_clients: 4,
        max_clients: 100,
        connection_usage_percent: 4,
        blocked_clients: 0,
        qps: 8,
        hit_rate: 0.5,
        keys: 1024,
        expired_keys_per_second: 0,
        evicted_keys_per_second: 0,
        rejected_connections_rate: 0,
        replication: {
          connected_replicas: null,
          master_link_up: false,
          master_last_io_seconds_ago: 30,
          master_sync_in_progress: false,
          worst_replica_lag_seconds: 7,
        },
        uptime_seconds: 90180,
        status: "critical",
        status_source: "replication",
        collection_level: "normal",
      },
      {
        id: "fixture-d",
        address: "192.0.2.43:6379",
        availability: "up",
        role: "unknown",
        cluster_enabled: true,
        used_memory_bytes: null,
        max_memory_bytes: null,
        memory_usage_percent: null,
        connected_clients: null,
        max_clients: null,
        connection_usage_percent: null,
        blocked_clients: null,
        qps: null,
        hit_rate: null,
        keys: null,
        expired_keys_per_second: null,
        evicted_keys_per_second: null,
        rejected_connections_rate: null,
        replication: {
          connected_replicas: null,
          master_link_up: null,
          master_last_io_seconds_ago: null,
          master_sync_in_progress: null,
          worst_replica_lag_seconds: null,
        },
        uptime_seconds: 43912800,
        status: "unknown",
        status_source: "unknown",
        collection_level: "normal",
      },
    ],
    total: 4,
    page: 1,
    page_size: 20,
    total_pages: 1,
  },
  meta: {
    request_id: "fixture-request",
    stale: false,
    collected_at: "2026-08-01T08:00:00Z",
  },
};

function renderPage(entry = "/redis") {
  window.history.replaceState({}, "", entry);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  render(
    <QueryClientProvider client={client}>
      <BrowserRouter>
        <Routes>
          <Route path="/redis" element={<RedisPage />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>,
  );
  return client;
}

function mockPaginatedRedisRequests() {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    const url = new URL(raw, "http://localhost");
    requests.push(url);
    const pageSize = Number(url.searchParams.get("page_size"));
    const total = pageSize === 500 ? pageSize500Total : 64;
    return Promise.resolve(
      new Response(
        JSON.stringify({
          ...fixture,
          data: {
            ...fixture.data,
            page: Number(url.searchParams.get("page")),
            page_size: pageSize,
            total,
            total_pages: Math.ceil(total / pageSize),
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
  });
}

beforeEach(() => {
  requests.length = 0;
  vi.restoreAllMocks();
  vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    requests.push(new URL(raw, "http://localhost"));
    return Promise.resolve(
      new Response(JSON.stringify(fixture), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
  });
});

it("严格渲染 Redis 十三列及拆分后的指标语义", async () => {
  renderPage();
  expect(
    await screen.findByRole("heading", { name: "Redis 实例" }),
  ).toBeVisible();
  await screen.findByTitle("192.0.2.40:6379");
  expect(screen.getByRole("table")).toHaveClass(
    "host-table",
    "redis-table",
    "observability-table",
  );
  expect(
    screen
      .getAllByRole("columnheader")
      .map((value) => value.textContent),
  ).toEqual([
    "实例地址",
    "角色",
    "内存上限",
    "内存使用率",
    "连接",
    "阻塞连接",
    "QPS",
    "命中率",
    "key 总数",
    "复制链路",
    "延迟",
    "运行时间",
    "状态",
  ]);
  const rows = screen.getAllByRole("row").slice(1);
  const master = within(rows[0]).getAllByRole("cell");
  const healthySlave = within(rows[1]).getAllByRole("cell");
  const disconnectedSlave = within(rows[2]).getAllByRole("cell");
  const unknown = within(rows[3]).getAllByRole("cell");

  expect(master).toHaveLength(13);
  expect(master[2]).toHaveTextContent("2 GiB");
  expect(master[3]).toHaveTextContent("50.0%");
  expect(master[4]).toHaveTextContent("12/100");
  expect(master[4]).not.toHaveTextContent("阻塞");
  expect(master[5]).toHaveTextContent("1");
  expect(master[6]).toHaveTextContent("25.5");
  expect(master[7]).toHaveTextContent("75.0%");
  expect(master[9]).toHaveTextContent("—");
  expect(master[10]).toHaveTextContent("2s");
  expect(master[11]).toHaveTextContent("1分钟");
  expect(master[11].firstElementChild).toHaveAttribute("title", "1分钟");

  expect(healthySlave[2]).toHaveTextContent("未设置上限");
  expect(healthySlave[3]).toHaveTextContent("暂无数据");
  expect(healthySlave[9]).toHaveTextContent("正常");
  expect(healthySlave[10]).toHaveTextContent("—");
  expect(healthySlave[11]).toHaveTextContent("暂无数据");
  expect(healthySlave[11].firstElementChild).toHaveAttribute("title", "暂无数据");

  expect(disconnectedSlave[2]).toHaveTextContent("暂无数据");
  expect(disconnectedSlave[9]).toHaveTextContent("断开");
  expect(disconnectedSlave[10]).toHaveTextContent("7s");
  expect(disconnectedSlave[11].textContent).toBe("1天 1小时");
  expect(disconnectedSlave[11].firstElementChild).toHaveAttribute(
    "title",
    "1天 1小时 3分钟",
  );
  expect(unknown[9]).toHaveTextContent("未知");
  expect(unknown[11].textContent).toBe("1年 143天");
  expect(unknown[11].firstElementChild).toHaveAttribute(
    "title",
    "1年 143天 6小时",
  );
  expect(
    screen.queryByRole("columnheader", { name: /过期|淘汰/ }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: /切换|故障转移|重启|删除|执行命令/ }),
  ).not.toBeInTheDocument();
});

it.each(redisSortFields)("从 fresh page 3 排序 Redis %s 时首击升序、再击降序，并发送精确参数", async (label, sort) => {
  const user = userEvent.setup();
  const initialSort = sort === "status" ? "instance" : "status";
  mockPaginatedRedisRequests();
  renderPage(
    `/redis?role=slave&status=warning&sort=${initialSort}&order=desc&page=3&page_size=20`,
  );
  await screen.findByText("192.0.2.40:6379");

  const button = screen.getByRole("button", {
    name: `${label}排序，当前未排序`,
  });
  expect(button).toHaveAttribute("data-active", "false");
  expect(button).toHaveAttribute("title", `${label}排序，当前未排序`);
  expect(button).not.toHaveTextContent(/[⇅↑↓]/);
  await user.click(button);
  await waitFor(() => {
    const ascending = screen.getByRole("button", {
      name: `${label}排序，当前升序`,
    });
    expect(ascending).toHaveAttribute("data-active", "true");
    expect(ascending).toHaveAttribute("title", `${label}排序，当前升序`);
    const parameters = new URLSearchParams(window.location.search);
    expect(parameters.get("sort")).toBe(sort);
    expect(parameters.get("order")).toBe("asc");
    expect(parameters.get("page")).toBe("1");
    expect(Object.fromEntries(requests.at(-1)!.searchParams)).toEqual({
      role: "slave",
      status: "warning",
      sort,
      order: "asc",
      page: "1",
      page_size: "20",
    });
  });

  await user.click(
    screen.getByRole("button", { name: `${label}排序，当前升序` }),
  );
  await waitFor(() => {
    const descending = screen.getByRole("button", {
      name: `${label}排序，当前降序`,
    });
    expect(descending).toHaveAttribute("data-active", "true");
    expect(descending).toHaveAttribute("title", `${label}排序，当前降序`);
    const parameters = new URLSearchParams(window.location.search);
    expect(parameters.get("sort")).toBe(sort);
    expect(parameters.get("order")).toBe("desc");
    expect(parameters.get("page")).toBe("1");
    expect(Object.fromEntries(requests.at(-1)!.searchParams)).toEqual({
      role: "slave",
      status: "warning",
      sort,
      order: "desc",
      page: "1",
      page_size: "20",
    });
  });
});

it("将 evicted 排序规范为 instance 且不发送 evicted", async () => {
  renderPage("/redis?sort=evicted&order=desc&page=1&page_size=50");
  await screen.findByText("192.0.2.40:6379");

  await waitFor(() => {
    const parameters = new URLSearchParams(window.location.search);
    expect(parameters.get("sort")).toBe("instance");
    expect(parameters.get("order")).toBe("desc");
    expect(Object.fromEntries(requests.at(-1)!.searchParams)).toEqual({
      sort: "instance",
      order: "desc",
      page: "1",
      page_size: "50",
    });
    expect(requests.some((request) => request.searchParams.get("sort") === "evicted")).toBe(false);
  });
});

it("复用现有列表控制栏并展示最新数据时间", async () => {
  renderPage();
  const search = await screen.findByRole("searchbox", { name: "搜索实例地址" });
  const controls = search.closest(".redis-list-controls");
  expect(controls).not.toBeNull();
  if (!(controls instanceof HTMLElement)) {
    throw new Error("Redis 控制区未渲染为 HTML 元素");
  }
  expect(controls).toHaveClass("host-list-controls", "mysql-list-controls");
  expect(search.closest(".host-search")).not.toBeNull();
  expect(within(controls).getAllByRole("combobox")).toHaveLength(3);
  const dataTime = await within(controls).findByText("2026/08/01 08:00:00");
  expect(dataTime.closest(".data-time")).toHaveTextContent("最新数据时间：2026/08/01 08:00:00");
  expect(within(controls).queryByRole("button", { name: /刷新/ })).not.toBeInTheDocument();
  expect(within(controls).queryByText(/上次刷新|自动刷新/)).not.toBeInTheDocument();
});

it("把角色状态排序和分页写入 URL 与固定 GET 参数", async () => {
  mockPaginatedRedisRequests();
  const user = userEvent.setup();
  renderPage("/redis?page=3&page_size=500");
  expect(await screen.findByText("第 3 / 3 页，共 1001 个实例")).toBeVisible();
  expect(screen.getByRole("combobox", { name: "每页数量" })).toHaveValue("500");
  expect(Object.fromEntries(requests.at(-1)!.searchParams)).toEqual({
    sort: "instance",
    order: "asc",
    page: "3",
    page_size: "500",
  });
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Redis 角色" }),
    "slave",
  );
  await waitFor(() => expect(window.location.search).toContain("role=slave"));
  await user.selectOptions(
    screen.getByRole("combobox", { name: "实例状态" }),
    "warning",
  );
  await user.click(screen.getByRole("button", { name: /内存使用率排序/ }));
  await waitFor(() => {
    expect(Object.fromEntries(requests.at(-1)!.searchParams)).toEqual({
      role: "slave",
      status: "warning",
      sort: "memory",
      order: "asc",
      page: "1",
      page_size: "500",
    });
  });
});

it("通过每页数量下拉切换到 500 并发送最后 GET", async () => {
  mockPaginatedRedisRequests();
  const user = userEvent.setup();
  renderPage("/redis?page=3&page_size=20");

  await screen.findByText("第 3 / 4 页，共 64 个实例");
  await user.selectOptions(
    screen.getByRole("combobox", { name: "每页数量" }),
    "500",
  );
  await waitFor(() => {
    expect(window.location.search).toContain("page=1&page_size=500");
    expect(Object.fromEntries(requests.at(-1)!.searchParams)).toEqual({
      sort: "instance",
      order: "asc",
      page: "1",
      page_size: "500",
    });
  });
});

it("500 条单页保留页数和总数文本但不渲染翻页按钮", async () => {
  pageSize500Total = 500;
  mockPaginatedRedisRequests();
  renderPage("/redis?page=1&page_size=500");

  expect(await screen.findByText("第 1 / 1 页，共 500 个实例")).toBeVisible();
  expect(screen.queryByRole("button", { name: "上一页" })).toBeNull();
  expect(screen.queryByRole("button", { name: "下一页" })).toBeNull();
});

it("搜索防抖后重置页码并仅发送固定参数", async () => {
  const user = userEvent.setup();
  renderPage("/redis?page=3");
  await screen.findByText("192.0.2.40:6379");

  await user.type(
    screen.getByRole("searchbox", { name: "搜索实例地址" }),
    "cache",
  );
  expect(window.location.search).not.toContain("search=cache");
  await waitFor(() => expect(window.location.search).toContain("search=cache"));
  expect(window.location.search).toContain("page=1");
  await waitFor(() =>
    expect(
      requests.some(
        (request) => request.searchParams.get("search") === "cache",
      ),
    ).toBe(true),
  );
});

it("展示分页并按服务端响应归一化越界页码", async () => {
  vi.mocked(globalThis.fetch).mockImplementation((input) => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    const url = new URL(raw, "http://localhost");
    requests.push(url);
    const requestedPage = Number(url.searchParams.get("page"));
    const responsePage = Math.min(Math.max(requestedPage, 1), 3);
    return Promise.resolve(
      new Response(
        JSON.stringify({
          ...fixture,
          data: {
            ...fixture.data,
            page: responsePage,
            total: 42,
            total_pages: 3,
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
  });

  const user = userEvent.setup();
  renderPage("/redis?page=99");
  await screen.findByText("第 3 / 3 页，共 42 个实例");
  await waitFor(() => expect(window.location.search).toContain("page=3"));
  expect(await screen.findByRole("button", { name: "下一页" })).toBeDisabled();
  await user.click(await screen.findByRole("button", { name: "上一页" }));
  await waitFor(() => expect(window.location.search).toContain("page=2"));
});

it.each([499, 501])("将非法 page_size=%i 规范为 20 且回到第一页", async (pageSize) => {
  mockPaginatedRedisRequests();
  renderPage(`/redis?page=3&page_size=${pageSize}`);

  await screen.findByText("192.0.2.40:6379");
  await waitFor(() => {
    expect(window.location.search).toContain("page=1&page_size=20");
    expect(requests.at(-1)?.searchParams.get("page_size")).toBe("20");
  });
});

it("展示初始加载和空列表状态", async () => {
  let resolveResponse: ((response: Response) => void) | undefined;
  vi.mocked(globalThis.fetch).mockImplementation(
    () =>
      new Promise((resolve) => {
        resolveResponse = resolve;
      }),
  );
  renderPage();
  expect(screen.getByText("正在加载 Redis 实例…")).toBeVisible();

  resolveResponse?.(
    new Response(
      JSON.stringify({
        ...fixture,
        data: {
          instances: [],
          total: 0,
          page: 1,
          page_size: 20,
          total_pages: 0,
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ),
  );
  expect(await screen.findByText("没有符合条件的 Redis 实例")).toBeVisible();
});

it("后台刷新失败时保留上一次列表并显示非阻断错误", async () => {
  const queryClient = renderPage();
  await screen.findByText("192.0.2.40:6379");
  vi.mocked(globalThis.fetch).mockResolvedValueOnce(
    new Response(
      JSON.stringify({
        code: "redis_unavailable",
        message: "Redis 数据暂时不可用",
        request_id: "fixture-error",
        retryable: true,
      }),
      { status: 503, headers: { "Content-Type": "application/json" } },
    ),
  );

  await queryClient.invalidateQueries({ queryKey: ["redis-instances"] });
  expect(await screen.findByText("Redis 数据暂时不可用")).toBeVisible();
  expect(screen.getByText("192.0.2.40:6379")).toBeVisible();
});
