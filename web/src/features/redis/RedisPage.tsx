import { useQuery } from "@tanstack/react-query";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { APIError, apiRequest } from "../../api/client";
import type {
  MetricLevel,
  RedisInstance,
  RedisInstancePageResponse,
  RedisRole,
} from "../../api/types";
import { useRefreshIntervalMs } from "../../app/runtime";
import { ErrorPanel } from "../../components/ErrorPanel";
import {
  ListPageControls,
  ListPageHeader,
  ListPageSizeField,
  ListSearchField,
  ListSelectField,
  ListTablePanel,
} from "../../components/ListPage";
import { StaleBanner } from "../../components/StaleBanner";
import { formatDurationDisplay } from "../../formatters/duration";

const pageSizes = [20, 50, 100, 500] as const;
const sortFields = [
  "instance",
  "role",
  "memory_limit",
  "memory",
  "connections",
  "blocked_connections",
  "qps",
  "hit_rate",
  "keys",
  "replication_link",
  "replication_lag",
  "uptime",
  "status",
] as const;

type RedisSort = (typeof sortFields)[number];
type PageSize = (typeof pageSizes)[number];

const levels: MetricLevel[] = ["normal", "warning", "critical", "unknown"];
const roleText: Record<RedisRole, string> = {
  master: "主节点",
  slave: "从节点",
  unknown: "未知",
};
const sourceText = {
  availability: "可用性",
  replication: "复制",
  memory: "内存",
  connection: "连接",
  collection: "采集",
  normal: "正常",
  unknown: "未知",
} as const;
const levelText: Record<MetricLevel, string> = {
  normal: "正常",
  warning: "警告",
  critical: "严重",
  unknown: "未知",
};

function decimal(value: number | null, digits = 2) {
  if (value === null) return "暂无数据";
  if (digits === 0) return value.toFixed(0);
  return value
    .toFixed(digits)
    .replace(/(\.\d*?)0+$/, "$1")
    .replace(/\.$/, "");
}

function percentage(value: number | null) {
  return value === null ? "暂无数据" : `${value.toFixed(1)}%`;
}

function byteSize(value: number | null) {
  if (value === null) return "暂无数据";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let unitIndex = 0;
  let size = value;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  return `${size.toFixed(1).replace(/\.0$/, "")} ${units[unitIndex]}`;
}

function connections(instance: RedisInstance) {
  if (instance.connected_clients === null && instance.max_clients === null) {
    return "暂无数据";
  }
  return `${decimal(instance.connected_clients, 0)}/${decimal(instance.max_clients, 0)}`;
}

function memoryLimit(value: number | null) {
  if (value === null) return "暂无数据";
  return value <= 0 ? "未设置上限" : byteSize(value);
}

function replicationLink(instance: RedisInstance) {
  if (instance.role === "master") return "—";
  if (instance.role === "unknown") return "未知";
  if (instance.replication.master_link_up === null) return "未知";
  return instance.replication.master_link_up ? "正常" : "断开";
}

function replicationLag(instance: RedisInstance) {
  const lag = instance.replication.worst_replica_lag_seconds;
  return lag === null ? "—" : `${decimal(lag)}s`;
}

function redisSort(value: string | null): RedisSort {
  return sortFields.includes(value as RedisSort)
    ? (value as RedisSort)
    : "instance";
}

function positivePage(value: string | null) {
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : 1;
}

function redisPageSize(value: string | null): PageSize {
  const parsed = Number(value);
  return pageSizes.includes(parsed as PageSize) ? (parsed as PageSize) : 20;
}

function redisRole(value: string | null): RedisRole | "" {
  return value === "master" || value === "slave" || value === "unknown"
    ? value
    : "";
}

function redisStatus(value: string | null): MetricLevel | "" {
  return levels.includes(value as MetricLevel) ? (value as MetricLevel) : "";
}

function statusLabel(instance: RedisInstance) {
  if (instance.collection_level === "critical") return "采集失联";
  if (instance.collection_level === "warning") return "采集延迟";
  if (instance.status_source === "normal") return levelText[instance.status];
  return sourceText[instance.status_source];
}

export function RedisPage() {
  const refreshIntervalMs = useRefreshIntervalMs();
  const [searchParams, setSearchParams] = useSearchParams();
  const querySearch = searchParams.get("search") ?? "";
  const role = redisRole(searchParams.get("role"));
  const status = redisStatus(searchParams.get("status"));
  const sort = redisSort(searchParams.get("sort"));
  const order = searchParams.get("order") === "desc" ? "desc" : "asc";
  const requestedPageSize = searchParams.get("page_size");
  const pageSize = redisPageSize(requestedPageSize);
  const page =
    requestedPageSize !== null && !pageSizes.includes(Number(requestedPageSize) as PageSize)
      ? 1
      : positivePage(searchParams.get("page"));
  const [searchText, setSearchText] = useState(querySearch);

  useEffect(() => {
    const canonical = new URLSearchParams(searchParams);
    if (querySearch === "") canonical.delete("search");
    else canonical.set("search", querySearch);
    if (role === "") canonical.delete("role");
    else canonical.set("role", role);
    if (status === "") canonical.delete("status");
    else canonical.set("status", status);
    canonical.set("sort", sort);
    canonical.set("order", order);
    canonical.set("page", String(page));
    canonical.set("page_size", String(pageSize));
    if (canonical.toString() !== searchParams.toString()) {
      setSearchParams(canonical, { replace: true });
    }
  }, [
    order,
    page,
    pageSize,
    querySearch,
    role,
    searchParams,
    setSearchParams,
    sort,
    status,
  ]);

  useEffect(() => setSearchText(querySearch), [querySearch]);

  useEffect(() => {
    if (searchText === querySearch) return;
    const timeout = window.setTimeout(() => {
      const next = new URLSearchParams(searchParams);
      if (searchText === "") next.delete("search");
      else next.set("search", searchText);
      next.set("page", "1");
      setSearchParams(next);
    }, 300);
    return () => window.clearTimeout(timeout);
  }, [querySearch, searchParams, searchText, setSearchParams]);

  const requestParameters = new URLSearchParams({
    sort,
    order,
    page: String(page),
    page_size: String(pageSize),
  });
  if (querySearch !== "") requestParameters.set("search", querySearch);
  if (role !== "") requestParameters.set("role", role);
  if (status !== "") requestParameters.set("status", status);

  const instances = useQuery({
    queryKey: [
      "redis-instances",
      querySearch,
      role,
      status,
      sort,
      order,
      page,
      pageSize,
    ],
    queryFn: ({ signal }) =>
      apiRequest<RedisInstancePageResponse>(
        `/api/v1/redis/instances?${requestParameters.toString()}`,
        { method: "GET", signal },
      ),
    refetchInterval: refreshIntervalMs,
    refetchIntervalInBackground: false,
  });

  const responsePage = instances.data?.data.page;
  const responseTotalPages = instances.data?.data.total_pages;
  const canonicalResponsePage =
    responsePage === undefined || responseTotalPages === undefined
      ? page
      : responseTotalPages === 0
        ? 1
        : Math.min(Math.max(responsePage, 1), responseTotalPages);

  useEffect(() => {
    if (instances.data === undefined || canonicalResponsePage === page) return;
    const next = new URLSearchParams(searchParams);
    next.set("page", String(canonicalResponsePage));
    setSearchParams(next, { replace: true });
  }, [
    canonicalResponsePage,
    instances.data,
    page,
    searchParams,
    setSearchParams,
  ]);

  function updateParameter(key: string, value: string, resetPage = true) {
    const next = new URLSearchParams(searchParams);
    if (value === "") next.delete(key);
    else next.set(key, value);
    if (resetPage) next.set("page", "1");
    setSearchParams(next);
  }

  function sortButton(field: RedisSort, label: string) {
    const current =
      sort === field ? (order === "asc" ? "升序" : "降序") : "未排序";
    const description = `${label}排序，当前${current}`;
    return (
      <button
        className="host-sort-button"
        type="button"
        data-active={sort === field}
        aria-label={description}
        title={description}
        onClick={() => {
          const next = new URLSearchParams(searchParams);
          next.set("sort", field);
          next.set("order", sort === field && order === "asc" ? "desc" : "asc");
          next.set("page", "1");
          setSearchParams(next);
        }}
      >
        {label}
      </button>
    );
  }

  const columns: ColumnDef<RedisInstance>[] = [
    {
      id: "instance",
      header: () => sortButton("instance", "实例地址"),
      cell: ({ row }) => (
        <span title={row.original.address}>{row.original.address}</span>
      ),
    },
    {
      id: "role",
      header: () => sortButton("role", "角色"),
      cell: ({ row }) => roleText[row.original.role],
    },
    {
      id: "memory-limit",
      header: () => sortButton("memory_limit", "内存上限"),
      cell: ({ row }) => memoryLimit(row.original.max_memory_bytes),
    },
    {
      id: "memory-usage",
      header: () => sortButton("memory", "内存使用率"),
      cell: ({ row }) => percentage(row.original.memory_usage_percent),
    },
    {
      id: "connections",
      header: () => sortButton("connections", "连接"),
      cell: ({ row }) => connections(row.original),
    },
    {
      id: "blocked-connections",
      header: () => sortButton("blocked_connections", "阻塞连接"),
      cell: ({ row }) => decimal(row.original.blocked_clients, 0),
    },
    {
      id: "qps",
      header: () => sortButton("qps", "QPS"),
      cell: ({ row }) => decimal(row.original.qps),
    },
    {
      id: "hit-rate",
      header: () => sortButton("hit_rate", "命中率"),
      cell: ({ row }) =>
        row.original.hit_rate === null
          ? "暂无数据"
          : percentage(row.original.hit_rate * 100),
    },
    {
      id: "keys",
      header: () => sortButton("keys", "key 总数"),
      cell: ({ row }) => decimal(row.original.keys, 0),
    },
    {
      id: "replication-link",
      header: () => sortButton("replication_link", "复制链路"),
      cell: ({ row }) => replicationLink(row.original),
    },
    {
      id: "replication-lag",
      header: () => sortButton("replication_lag", "延迟"),
      cell: ({ row }) => replicationLag(row.original),
    },
    {
      id: "uptime",
      header: () => sortButton("uptime", "运行时间"),
      cell: ({ row }) => {
        const value = formatDurationDisplay(row.original.uptime_seconds);
        return <span title={value.title}>{value.text}</span>;
      },
    },
    {
      id: "status",
      header: () => sortButton("status", "状态"),
      cell: ({ row }) => {
        const instance = row.original;
        const effectiveLevel =
          instance.collection_level === "warning" ||
          instance.collection_level === "critical"
            ? instance.collection_level
            : instance.status;
        return (
          <span className="status-badge" data-level={effectiveLevel}>
            <span className="status-badge-dot" aria-hidden="true" />
            {statusLabel(instance)}
          </span>
        );
      },
    },
  ];
  const table = useReactTable({
    data: instances.data?.data.instances ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  });
  const apiError = instances.error instanceof APIError ? instances.error : null;
  const hasData = instances.data !== undefined;

  return (
    <section className="redis-page" aria-labelledby="redis-title">
      <ListPageHeader
        eyebrow="缓存观测"
        title="Redis 实例"
        description="只读展示节点、性能与复制状态。"
        titleId="redis-title"
      />

      <ListPageControls
        className="mysql-list-controls redis-list-controls"
        collectedAt={instances.data?.meta.collected_at}
      >
        <ListSearchField
          label="搜索实例地址"
          value={searchText}
          onChange={(event) => setSearchText(event.target.value)}
        />
        <ListSelectField
          label="Redis 角色"
          value={role}
          onChange={(event) => updateParameter("role", event.target.value)}
          options={[
            { value: "", label: "全部角色" },
            { value: "master", label: "主节点" },
            { value: "slave", label: "从节点" },
            { value: "unknown", label: "未知" },
          ]}
        />
        <ListSelectField
          label="实例状态"
          value={status}
          onChange={(event) => updateParameter("status", event.target.value)}
          options={[
            { value: "", label: "全部状态" },
            ...levels.map((level) => ({
              value: level,
              label: levelText[level],
            })),
          ]}
        />
        <ListPageSizeField
          value={pageSize}
          onChange={(event) =>
            updateParameter("page_size", event.target.value)
          }
          pageSizes={pageSizes}
        />
      </ListPageControls>

      {instances.data?.meta.stale === true &&
        instances.data.meta.collected_at !== undefined && (
          <StaleBanner collectedAt={instances.data.meta.collected_at} />
        )}

      {instances.isError && (
        <ErrorPanel
          title={hasData ? "Redis 实例列表刷新失败" : "Redis 实例列表加载失败"}
          message={apiError?.message ?? "暂时无法加载 Redis 实例"}
          retryable={apiError?.retryable ?? true}
          retryLabel="重试 Redis 实例列表"
          onRetry={() => void instances.refetch()}
        />
      )}

      {!hasData && instances.isPending ? (
        <div className="host-list-loading">正在加载 Redis 实例…</div>
      ) : hasData ? (
        <ListTablePanel
          scrollClassName="redis-table-scroll"
          emptyState={
            instances.data.data.instances.length === 0 ? (
              <div className="host-empty">没有符合条件的 Redis 实例</div>
            ) : undefined
          }
          paginationLabel="Redis 实例列表分页"
          pagination={
            <>
            <span>
              {instances.data.data.total_pages === 0
                ? "暂无实例"
                : `第 ${instances.data.data.page} / ${instances.data.data.total_pages} 页，共 ${instances.data.data.total} 个实例`}
            </span>
            {instances.data.data.total_pages > 1 && <div>
              <button
                className="secondary-button"
                type="button"
                disabled={
                  instances.data.data.total_pages === 0 ||
                  instances.data.data.page <= 1
                }
                onClick={() =>
                  updateParameter(
                    "page",
                    String(Math.max(instances.data!.data.page - 1, 1)),
                    false,
                  )
                }
              >
                上一页
              </button>
              <button
                className="secondary-button"
                type="button"
                disabled={
                  instances.data.data.total_pages === 0 ||
                  instances.data.data.page >= instances.data.data.total_pages
                }
                onClick={() =>
                  updateParameter(
                    "page",
                    String(
                      Math.min(
                        instances.data!.data.page + 1,
                        instances.data!.data.total_pages,
                      ),
                    ),
                    false,
                  )
                }
              >
                下一页
              </button>
            </div>}
            </>
          }
        >
          <table className="host-table redis-table observability-table">
            <thead>
              {table.getHeaderGroups().map((group) => (
                <tr key={group.id}>
                  {group.headers.map((header) => (
                    <th key={header.id}>
                      {flexRender(
                        header.column.columnDef.header,
                        header.getContext(),
                      )}
                    </th>
                  ))}
                </tr>
              ))}
            </thead>
            <tbody>
              {table.getRowModel().rows.map((row) => (
                <tr key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id}>
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext(),
                      )}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </ListTablePanel>
      ) : null}
    </section>
  );
}
