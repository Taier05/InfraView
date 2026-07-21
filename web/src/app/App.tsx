import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  BrowserRouter,
  Navigate,
  Route,
  Routes,
} from 'react-router-dom'

import { AuthProvider, useAuth } from '../auth/AuthProvider'
import { LoginPage } from '../auth/LoginPage'
import { AppShell } from './AppShell'
import './theme.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: false,
    },
  },
})

function ProtectedShell() {
  const { status } = useAuth()

  if (status === 'loading') {
    return (
      <div className="loading-screen" role="status">
        正在验证登录状态…
      </div>
    )
  }

  if (status === 'unauthenticated') {
    return <Navigate to="/login" replace />
  }

  return <AppShell />
}

function LoginRoute() {
  const { status } = useAuth()
  if (status === 'loading') {
    return (
      <div className="loading-screen" role="status">
        正在验证登录状态…
      </div>
    )
  }
  if (status === 'authenticated') return <Navigate to="/" replace />
  return <LoginPage />
}

function OverviewPage() {
  return (
    <section aria-labelledby="overview-title">
      <p className="eyebrow">运行态势</p>
      <h1 id="overview-title">基础设施总览</h1>
      <p className="page-description">集中查看主机健康、资源用量与近期趋势。</p>
      <div className="empty-panel">概览数据将在此展示</div>
    </section>
  )
}

function HostsPage() {
  return (
    <section aria-labelledby="hosts-title">
      <p className="eyebrow">资产清单</p>
      <h1 id="hosts-title">主机</h1>
      <p className="page-description">查看纳入观测范围的 Linux 主机。</p>
      <div className="empty-panel">主机列表将在此展示</div>
    </section>
  )
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<LoginRoute />} />
            <Route element={<ProtectedShell />}>
              <Route index element={<OverviewPage />} />
              <Route path="hosts" element={<HostsPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
