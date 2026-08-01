import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import {
  ListPageControls,
  ListPageHeader,
  ListPageSizeField,
  ListSearchField,
  ListSelectField,
  ListTablePanel,
} from "./ListPage";

function ControlsFixture() {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [pageSize, setPageSize] = useState("20");

  return (
    <>
      <ListPageHeader
        eyebrow="缓存观测"
        title="Redis 实例"
        description="只读展示 Redis 状态。"
        titleId="redis-title"
      />
      <ListPageControls
        className="redis-list-controls"
        refresh={{
          isFetching: false,
          dataUpdatedAt: Date.now(),
          onRefresh: vi.fn(),
          refreshIntervalSeconds: 15,
          ariaLabel: "刷新 Redis 实例列表",
        }}
      >
        <ListSearchField
          label="搜索实例地址"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <ListSelectField
          label="实例状态"
          value={status}
          onChange={(event) => setStatus(event.target.value)}
          options={[
            { value: "", label: "全部状态" },
            { value: "normal", label: "正常" },
          ]}
        />
        <ListPageSizeField
          value={pageSize}
          onChange={(event) => setPageSize(event.target.value)}
          pageSizes={[20, 50, 100]}
        />
      </ListPageControls>
    </>
  );
}

describe("ListPage shared template", () => {
  it("keeps labelled fields and refresh status in one control bar", async () => {
    const user = userEvent.setup();
    render(<ControlsFixture />);

    expect(
      screen.getByRole("heading", { name: "Redis 实例" }),
    ).toHaveAttribute("id", "redis-title");
    const search = screen.getByRole("searchbox", { name: "搜索实例地址" });
    await user.type(search, "cache");
    expect(search).toHaveValue("cache");

    const controls = search.closest(".host-list-controls");
    expect(controls).toHaveClass("redis-list-controls");
    if (!(controls instanceof HTMLElement)) {
      throw new Error("共享控制栏未渲染为 HTML 元素");
    }
    expect(
      within(controls).getByRole("combobox", { name: "实例状态" }),
    ).toBeVisible();
    expect(
      within(controls).getByRole("button", { name: "刷新 Redis 实例列表" }),
    ).toBeVisible();
    expect(within(controls).getByText(/自动刷新/)).toBeVisible();
    for (const label of ["20 条", "50 条", "100 条"]) {
      expect(within(controls).getByRole("option", { name: label })).toBeVisible();
    }
  });

  it("keeps table empty state and pagination in one panel", () => {
    render(
      <ListTablePanel
        scrollClassName="redis-table-scroll"
        emptyState={<div className="host-empty">暂无实例</div>}
        pagination={<span>第 1 / 1 页</span>}
        paginationLabel="Redis 实例列表分页"
      >
        <table aria-label="Redis 实例列表">
          <tbody>
            <tr>
              <td>实例</td>
            </tr>
          </tbody>
        </table>
      </ListTablePanel>,
    );

    const panel = screen.getByRole("table", { name: "Redis 实例列表" }).closest(
      ".host-table-panel",
    );
    expect(panel).not.toBeNull();
    if (!(panel instanceof HTMLElement)) {
      throw new Error("共享表格面板未渲染为 HTML 元素");
    }
    expect(within(panel).getByText("暂无实例")).toBeVisible();
    expect(
      within(panel).getByLabelText("Redis 实例列表分页"),
    ).toHaveTextContent("第 1 / 1 页");
  });
});
