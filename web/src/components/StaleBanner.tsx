import { DataTime } from './DataTime'

export interface StaleBannerProps {
  collectedAt: string
}

export function StaleBanner({ collectedAt }: StaleBannerProps) {
  return (
    <div className="stale-banner" role="alert">
      <strong>数据已过期</strong>
      <span>
        当前展示最近一次可用数据，
        <DataTime collectedAt={collectedAt} label="最新数据时间：" />
      </span>
    </div>
  )
}
