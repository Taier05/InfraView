interface RefreshControlProps {
  isFetching: boolean
  dataUpdatedAt: number
  onRefresh: () => void
  ariaLabel?: string
}

function refreshTime(timestamp: number) {
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(timestamp)
}

export function RefreshControl({
  isFetching,
  dataUpdatedAt,
  onRefresh,
  ariaLabel = '刷新',
}: RefreshControlProps) {
  return (
    <div className="refresh-control">
      <button
        className="secondary-button refresh-button"
        type="button"
        aria-label={ariaLabel}
        disabled={isFetching}
        onClick={onRefresh}
      >
        刷新
      </button>
      <span className="refresh-time" aria-live="polite">
        {isFetching
          ? '正在刷新…'
          : dataUpdatedAt > 0
            ? `上次刷新 ${refreshTime(dataUpdatedAt)}`
            : '等待首次刷新'}
        {' · 每 30 秒自动刷新'}
      </span>
    </div>
  )
}
