import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  BrowserRouter,
  Navigate,
  Route,
  Routes,
} from 'react-router-dom'

import { AuthProvider, useAuth } from '../auth/AuthProvider'
import { LoginPage } from '../auth/LoginPage'
import { DiskPage } from '../features/disks/DiskPage'
import { ElasticsearchPage } from '../features/elasticsearch/ElasticsearchPage'
import { HostListPage } from '../features/hosts/HostListPage'
import { JavaPage } from '../features/java/JavaPage'
import { MySQLPage } from '../features/mysql/MySQLPage'
import { OverviewPage } from '../features/overview/OverviewPage'
import { RabbitMQPage } from '../features/rabbitmq/RabbitMQPage'
import { RedisPage } from '../features/redis/RedisPage'
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

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<LoginRoute />} />
            <Route element={<ProtectedShell />}>
              <Route index element={<OverviewPage />} />
              <Route path="hosts" element={<HostListPage />} />
              <Route path="disks" element={<DiskPage />} />
              <Route path="mysql" element={<MySQLPage />} />
              <Route path="redis" element={<RedisPage />} />
              <Route path="elasticsearch" element={<ElasticsearchPage />} />
              <Route path="rabbitmq" element={<RabbitMQPage />} />
              <Route path="java" element={<JavaPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
