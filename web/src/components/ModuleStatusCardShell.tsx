import type { PropsWithChildren } from "react";
import { Link } from "react-router-dom";

import { DataTime } from "./DataTime";

type ModuleCardLevel =
  | "normal"
  | "warning"
  | "critical"
  | "unknown"
  | "empty";

interface ModuleEmptyState {
  title: string;
  description: string;
}

interface ModuleStatusCardShellProps extends PropsWithChildren {
  to: string;
  ariaLabel: string;
  category: string;
  title: string;
  level: ModuleCardLevel;
  levelLabel: string;
  actionLabel: string;
  emptyState?: ModuleEmptyState;
  className?: string;
  collectedAt?: string;
}

export function ModuleStatusCardShell({
  to,
  ariaLabel,
  category,
  title,
  level,
  levelLabel,
  actionLabel,
  emptyState,
  className,
  collectedAt,
  children,
}: ModuleStatusCardShellProps) {
  const classes = ["module-status-card", className].filter(Boolean).join(" ");
  const showEmptyState = level === "empty" && emptyState !== undefined;

  return (
    <Link
      className={classes}
      data-level={level}
      to={to}
      aria-label={ariaLabel}
    >
      <div className="module-status-heading">
        <div>
          <span>{category}</span>
          <h2>{title}</h2>
        </div>
        <span className="module-status-level" data-level={level}>
          {levelLabel}
        </span>
      </div>

      {showEmptyState ? (
        <div className="module-overview-empty-state">
          <strong>{emptyState.title}</strong>
          <span>{emptyState.description}</span>
        </div>
      ) : (
        children
      )}

      <div className="module-status-footer">
        <DataTime collectedAt={collectedAt} className="data-time" />
        <span className="module-status-action">
          {actionLabel} <span aria-hidden="true">→</span>
        </span>
      </div>
    </Link>
  );
}
