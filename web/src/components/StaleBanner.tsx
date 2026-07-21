export interface StaleBannerProps {
  collectedAt: string
}

export function StaleBanner({ collectedAt }: StaleBannerProps) {
  return (
    <div className="stale-banner" role="alert">
      <strong>数据已过期</strong>
      <span>
        当前展示最近一次可用数据，采集时间：
        <time dateTime={collectedAt}>{collectedAt}</time>
      </span>
    </div>
  )
}
