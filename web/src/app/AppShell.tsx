import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'

import { APIError } from '../api/client'
import { useAuth } from '../auth/AuthProvider'

export function AppShell() {
  const { logout, username } = useAuth()
  const queryClient = useQueryClient()
  const [logoutError, setLogoutError] = useState<string | null>(null)

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
    <div className="app-shell">
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
        </nav>

        <div className="source-status" aria-label="数据源状态">
          <span className="status-label">数据源</span>
          <strong>Mock</strong>
          <span className="status-detail">
            <span className="status-dot" aria-hidden="true" />
            等待状态数据
          </span>
        </div>
      </aside>

      <div className="workspace">
        <header className="topbar">
          <div>
            <span className="topbar-label">当前用户</span>
            <strong>{username}</strong>
          </div>
          <div className="toolbar" aria-label="页面控制">
            <label className="range-control">
              <span>时间范围</span>
              <select defaultValue="24h">
                <option value="1h">1小时</option>
                <option value="24h">24小时</option>
                <option value="7d">7天</option>
                <option value="30d">30天</option>
              </select>
            </label>
            <button
              className="secondary-button"
              type="button"
              onClick={() => void queryClient.invalidateQueries()}
            >
              刷新
            </button>
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
          <Outlet />
        </main>
      </div>
    </div>
  )
}
