export interface ErrorPanelProps {
  message: string
  retryable: boolean
  onRetry: () => void
}

export function ErrorPanel({ message, retryable, onRetry }: ErrorPanelProps) {
  return (
    <div className="error-panel" role="alert">
      <div>
        <strong>无法加载总览数据</strong>
        <p>{message}</p>
      </div>
      {retryable && (
        <button className="secondary-button" type="button" onClick={onRetry}>
          重试
        </button>
      )}
    </div>
  )
}
