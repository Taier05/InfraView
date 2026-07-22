import { useQueryClient } from '@tanstack/react-query'
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'

import { APIError, apiRequest, onUnauthorized } from '../api/client'
import type {
  ApiResponse,
  LoginCredentials,
  SessionData,
} from '../api/types'

type AuthStatus = 'loading' | 'authenticated' | 'unauthenticated'

interface AuthContextValue {
  status: AuthStatus
  username: string | null
  isLoggingIn: boolean
  login: (credentials: LoginCredentials) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [status, setStatus] = useState<AuthStatus>('loading')
  const statusRef = useRef<AuthStatus>('loading')
  const [username, setUsername] = useState<string | null>(null)
  const [isLoggingIn, setIsLoggingIn] = useState(false)

  function updateStatus(nextStatus: AuthStatus) {
    statusRef.current = nextStatus
    setStatus(nextStatus)
  }

  useEffect(
    () =>
      onUnauthorized(() => {
        if (statusRef.current !== 'authenticated') return
        statusRef.current = 'unauthenticated'
        queryClient.clear()
        setUsername(null)
        setStatus('unauthenticated')
      }),
    [queryClient],
  )

  useEffect(() => {
    const controller = new AbortController()

    void apiRequest<ApiResponse<SessionData>>('/api/v1/session', {
      signal: controller.signal,
    })
      .then((response) => {
        setUsername(response.data.username)
        updateStatus('authenticated')
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setUsername(null)
          updateStatus('unauthenticated')
        }
      })

    return () => controller.abort()
  }, [])

  async function login(credentials: LoginCredentials) {
    setIsLoggingIn(true)
    try {
      await apiRequest<void>('/api/v1/session', {
        method: 'POST',
        body: JSON.stringify(credentials),
      })
      const response = await apiRequest<ApiResponse<SessionData>>(
        '/api/v1/session',
      )
      setUsername(response.data.username)
      updateStatus('authenticated')
    } finally {
      setIsLoggingIn(false)
    }
  }

  async function logout() {
    await apiRequest<void>('/api/v1/session', { method: 'DELETE' })
    queryClient.clear()
    setUsername(null)
    updateStatus('unauthenticated')
  }

  const value = useMemo<AuthContextValue>(
    () => ({ status, username, isLoggingIn, login, logout }),
    [isLoggingIn, status, username],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (value === null) {
    throw new APIError(
      500,
      'auth_context_missing',
      '认证组件尚未初始化',
      '',
      false,
    )
  }
  return value
}
