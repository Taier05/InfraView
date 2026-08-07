import type {
  ChangeEventHandler,
  PropsWithChildren,
  ReactNode,
} from "react";

import { DataTime } from "./DataTime";

interface ListPageHeaderProps {
  eyebrow: string;
  title: string;
  description: string;
  titleId: string;
}

interface ListPageControlsProps extends PropsWithChildren {
  className?: string;
  collectedAt?: string;
}

interface ListSearchFieldProps {
  label: string;
  value: string;
  onChange: ChangeEventHandler<HTMLInputElement>;
}

export interface ListSelectOption {
  value: string;
  label: string;
}

interface ListSelectFieldProps {
  label: string;
  value: string;
  onChange: ChangeEventHandler<HTMLSelectElement>;
  options: readonly ListSelectOption[];
  className?: string;
}

interface ListPageSizeFieldProps {
  value: string | number;
  onChange: ChangeEventHandler<HTMLSelectElement>;
  pageSizes: readonly number[];
}

interface ListTablePanelProps extends PropsWithChildren {
  scrollClassName?: string;
  emptyState?: ReactNode;
  pagination?: ReactNode;
  paginationLabel: string;
}

function classes(...values: Array<string | undefined>) {
  return values.filter(Boolean).join(" ");
}

export function ListPageHeader({
  eyebrow,
  title,
  description,
  titleId,
}: ListPageHeaderProps) {
  return (
    <>
      <p className="eyebrow">{eyebrow}</p>
      <h1 id={titleId}>{title}</h1>
      <p className="page-description">{description}</p>
    </>
  );
}

export function ListPageControls({
  children,
  className,
  collectedAt,
}: ListPageControlsProps) {
  return (
    <div className={classes("host-list-controls", className)}>
      {children}
      <DataTime collectedAt={collectedAt} className="data-time" />
    </div>
  );
}

export function ListSearchField({
  label,
  value,
  onChange,
}: ListSearchFieldProps) {
  return (
    <label className="host-search">
      <span>{label}</span>
      <input type="search" value={value} onChange={onChange} />
    </label>
  );
}

export function ListSelectField({
  label,
  value,
  onChange,
  options,
  className = "host-status-filter",
}: ListSelectFieldProps) {
  return (
    <label className={className}>
      <span>{label}</span>
      <select value={value} onChange={onChange}>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}

export function ListPageSizeField({
  value,
  onChange,
  pageSizes,
}: ListPageSizeFieldProps) {
  return (
    <label className="host-page-size">
      <span>每页数量</span>
      <select value={value} onChange={onChange}>
        {pageSizes.map((size) => (
          <option key={size} value={size}>
            {size === 500 ? '全部（最多500条）' : `${size} 条`}
          </option>
        ))}
      </select>
    </label>
  );
}

export function ListTablePanel({
  children,
  scrollClassName,
  emptyState,
  pagination,
  paginationLabel,
}: ListTablePanelProps) {
  return (
    <div className="host-table-panel">
      <div className={classes("host-table-scroll", scrollClassName)}>
        {children}
        {emptyState}
      </div>
      {pagination !== undefined && (
        <div className="host-pagination" aria-label={paginationLabel}>
          {pagination}
        </div>
      )}
    </div>
  );
}
