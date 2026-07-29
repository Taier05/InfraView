import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'

import { APIError, apiRequest } from '../api/client'
import type { DataSourceStatusResponse } from '../api/types'
import { useAuth } from '../auth/AuthProvider'
import { refreshIntervalMilliseconds } from './runtime'

export function AppShell() {
  const { logout, username } = useAuth()
  const [logoutError, setLogoutError] = useState<string | null>(null)
  const datasource = useQuery({
    queryKey: ['datasource-status'],
    queryFn: ({ signal }) =>
      apiRequest<DataSourceStatusResponse>('/api/v1/datasource/status', {
        signal,
      }),
    refetchInterval: (query) =>
      refreshIntervalMilliseconds(
        query.state.data?.data.refresh_interval_seconds,
      ),
    refetchIntervalInBackground: false,
  })

  const datasourceData = datasource.data
  const datasourceState =
    datasourceData === undefined
      ? datasource.isPending
        ? 'loading'
        : 'error'
      : datasourceData.meta.stale
        ? 'stale'
        : datasourceData.data.healthy
          ? 'healthy'
          : 'unhealthy'
  const datasourceLabel =
    datasourceState === 'loading'
      ? '正在检查'
      : datasourceState === 'error'
        ? '请求失败'
        : datasourceState === 'stale'
          ? '状态过期'
          : datasourceState === 'healthy'
            ? '健康'
            : '异常'
  const datasourceName =
    datasourceData?.data.type === 'nightingale'
      ? 'Nightingale'
      : datasourceData?.data.type === 'mock'
        ? 'Mock'
        : '待确认'
  const connectionSummary =
    datasourceState === 'loading'
      ? '正在检查'
      : datasourceState === 'error'
        ? '状态获取失败'
        : datasourceState === 'stale'
          ? '状态过期'
          : datasourceState === 'unhealthy'
            ? '1 个连接异常'
            : datasourceData?.data.type === 'mock'
              ? '包含 Mock 数据'
              : '1/1 正常'
  const refreshIntervalMs = refreshIntervalMilliseconds(
    datasourceData?.data.refresh_interval_seconds,
  )

  async function handleLogout() {
    setLogoutError(null)
    try {
      await logout()
    } catch (cause) {
      setLogoutError(
        cause instanceof APIError
          ? cause.message
          : '退出登录失败，请稍后重试',
      )
    }
  }

  return (
    <div className="app-shell" data-density="compact">
      <a className="skip-link" href="#main-content">
        跳到主要内容
      </a>

      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">
            IV
          </span>
          <span>
            <strong>InfraView</strong>
            <small>基础设施观测</small>
          </span>
        </div>

        <nav aria-label="主导航">
          <NavLink to="/" end>
            总览
          </NavLink>
          <NavLink to="/hosts">主机</NavLink>
          <NavLink to="/mysql">MySQL</NavLink>
        </nav>

        <details
          className="source-status"
          aria-label="数据连接汇总"
          data-state={datasourceState}
          data-mode={datasourceData?.data.type ?? 'unknown'}
        >
          <summary>
            <span className="status-dot" aria-hidden="true" />
            <span className="connection-summary-title">数据连接</span>
            <span className="connection-summary-state">
              {connectionSummary}
            </span>
            <span className="connection-toggle" aria-hidden="true">
              ›
            </span>
          </summary>
          <div className="connection-details">
            <div className="connection-row">
              <span className="connection-kind">指标</span>
              <strong>{datasourceName}</strong>
              <span className="connection-health">{datasourceLabel}</span>
            </div>
            {datasourceData !== undefined && (
              <>
                {datasourceData.meta.stale && (
                  <span className="source-last-result">
                    上次检查{datasourceData.data.healthy ? '健康' : '异常'}
                  </span>
                )}
                <span className="source-checked-at">
                  最近检查
                  <time dateTime={datasourceData.data.checked_at}>
                    {new Intl.DateTimeFormat('zh-CN', {
                      dateStyle: 'short',
                      timeStyle: 'medium',
                    }).format(new Date(datasourceData.data.checked_at))}
                  </time>
                </span>
              </>
            )}
            {datasourceData !== undefined && datasource.isError && (
              <span className="source-request-error" role="alert">
                状态刷新失败
              </span>
            )}
          </div>
        </details>
      </aside>

      <div className="workspace" data-density="dense">
        <header className="topbar">
          <div>
            <span className="topbar-label">当前用户</span>
            <strong>{username}</strong>
          </div>
          <div className="toolbar" aria-label="页面控制">
            <button
              className="secondary-button"
              type="button"
              onClick={handleLogout}
            >
              退出登录
            </button>
          </div>
        </header>

        {logoutError !== null && (
          <p className="shell-error form-error" role="alert">
            {logoutError}
          </p>
        )}

        <main id="main-content" className="content" tabIndex={-1}>
          <Outlet context={{ refreshIntervalMs }} />
        </main>
      </div>
    </div>
  )
}
