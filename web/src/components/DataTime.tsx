interface DataTimeProps {
  collectedAt?: string;
  className?: string;
  label?: string;
}

function formatDataTime(value?: string) {
  if (!value) {
    return null;
  }

  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) {
    return null;
  }

  const parts = new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).formatToParts(date);
  const values = Object.fromEntries(
    parts.map((part) => [part.type, part.value]),
  );

  return `${values.year}/${values.month}/${values.day} ${values.hour}:${values.minute}:${values.second}`;
}

export function DataTime({
  collectedAt,
  className,
  label = "最新数据时间：",
}: DataTimeProps) {
  const formatted = formatDataTime(collectedAt);

  return (
    <span className={className}>
      {label}
      {formatted && collectedAt ? (
        <time dateTime={collectedAt}>{formatted}</time>
      ) : (
        "暂无数据"
      )}
    </span>
  );
}
