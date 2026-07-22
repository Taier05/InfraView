export interface ErrorPanelProps {
  title: string
  message: string
  retryable: boolean
  retryLabel: string
  onRetry: () => void
}

export function ErrorPanel({
  title,
  message,
  retryable,
  retryLabel,
  onRetry,
}: ErrorPanelProps) {
  return (
    <div className="error-panel" role="alert">
      <div>
        <strong>{title}</strong>
        <p>{message}</p>
      </div>
      {retryable && (
        <button className="secondary-button" type="button" onClick={onRetry}>
          {retryLabel}
        </button>
      )}
    </div>
  )
}
