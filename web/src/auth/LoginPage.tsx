import { useState, type FormEvent } from 'react'

import { APIError } from '../api/client'
import { useAuth } from './AuthProvider'

export function LoginPage() {
  const { isLoggingIn, login } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError(null)

    try {
      await login({ username, password })
    } catch (cause) {
      setPassword('')
      setError(
        cause instanceof APIError ? cause.message : '登录失败，请稍后重试',
      )
    }
  }

  return (
    <main className="login-page">
      <section className="login-panel" aria-labelledby="login-title">
        <div className="login-brand" aria-hidden="true">
          <span className="brand-mark">IV</span>
          <span>InfraView</span>
        </div>
        <div className="login-copy">
          <p className="eyebrow">只读基础设施观测</p>
          <h1 id="login-title">登录 InfraView</h1>
          <p>查看主机健康与资源趋势，不提供远程操作入口。</p>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
          <label htmlFor="username">用户名</label>
          <input
            id="username"
            name="username"
            autoComplete="username"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            required
          />

          <label htmlFor="password">密码</label>
          <input
            id="password"
            name="password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            required
          />

          {error !== null && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}

          <button className="primary-button" type="submit" disabled={isLoggingIn}>
            {isLoggingIn ? '正在登录…' : '登录'}
          </button>
        </form>
      </section>
    </main>
  )
}
