import { useOutletContext } from 'react-router-dom'

export const defaultRefreshIntervalMs = 15_000

export interface AppOutletContext {
  refreshIntervalMs: number
}

export function refreshIntervalMilliseconds(seconds: number | undefined) {
  return seconds !== undefined && seconds > 0
    ? seconds * 1_000
    : defaultRefreshIntervalMs
}

export function useRefreshIntervalMs() {
  const context = useOutletContext<AppOutletContext | null>()
  return context !== null && context.refreshIntervalMs > 0
    ? context.refreshIntervalMs
    : defaultRefreshIntervalMs
}
