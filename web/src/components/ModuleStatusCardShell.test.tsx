import { render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";

import { ModuleStatusCardShell } from "./ModuleStatusCardShell";

describe("ModuleStatusCardShell", () => {
  it("renders a module summary without owning its business content", () => {
    render(
      <MemoryRouter>
        <ModuleStatusCardShell
          to="/redis"
          ariaLabel="查看 Redis 板块"
          category="缓存板块"
          title="Redis"
          level="warning"
          levelLabel="存在警告"
          actionLabel="查看 Redis"
          className="redis-overview-card"
        >
          <div>业务告警摘要</div>
        </ModuleStatusCardShell>
      </MemoryRouter>,
    );

    const card = screen.getByRole("link", { name: "查看 Redis 板块" });
    expect(card).toHaveAttribute("href", "/redis");
    expect(card).toHaveClass("module-status-card", "redis-overview-card");
    expect(card).toHaveAttribute("data-level", "warning");
    expect(within(card).getByText("缓存板块")).toBeVisible();
    expect(within(card).getByRole("heading", { name: "Redis" })).toBeVisible();
    expect(within(card).getByText("存在警告")).toBeVisible();
    expect(within(card).getByText("业务告警摘要")).toBeVisible();
    expect(within(card).getByText("查看 Redis")).toBeVisible();
  });

  it("renders the shared empty state instead of metric children", () => {
    render(
      <MemoryRouter>
        <ModuleStatusCardShell
          to="/redis"
          ariaLabel="查看 Redis 板块"
          category="缓存板块"
          title="Redis"
          level="empty"
          levelLabel="暂无实例"
          actionLabel="查看 Redis"
          emptyState={{
            title: "暂无 Redis 实例",
            description: "尚无可展示的实例健康数据",
          }}
        >
          <div>不应显示的指标</div>
        </ModuleStatusCardShell>
      </MemoryRouter>,
    );

    const card = screen.getByRole("link", { name: "查看 Redis 板块" });
    expect(card).toHaveAttribute("data-level", "empty");
    expect(within(card).getByText("暂无 Redis 实例")).toBeVisible();
    expect(within(card).getByText("尚无可展示的实例健康数据")).toBeVisible();
    expect(within(card).queryByText("不应显示的指标")).not.toBeInTheDocument();
  });
});
