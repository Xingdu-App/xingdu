import { t } from "./i18n";
import "./LoadingSkeleton.css";

export function Skeleton({ wide = false }: { wide?: boolean }) {
  return (
    <span
      className={`skeleton${wide ? " skeleton-wide" : ""}`}
      aria-hidden="true"
    />
  );
}

export function TableSkeleton({
  headers,
  columns = 6,
}: {
  headers?: string[];
  columns?: number;
}) {
  const count = headers?.length ?? columns;
  return (
    <div
      className="table-scroll skeleton-table"
      role="status"
      aria-label={t("加载中…")}
      aria-busy="true"
    >
      <table aria-hidden="true">
        <thead>
          <tr>
            {Array.from({ length: count }, (_, i) => (
              <th key={i}>{headers?.[i] ?? <Skeleton />}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {Array.from({ length: 5 }, (_, row) => (
            <tr key={row}>
              {Array.from({ length: count }, (_, col) => (
                <td key={col}>
                  <Skeleton wide={(row + col) % 3 === 0} />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function CardSkeleton() {
  return (
    <div
      className="node-grid"
      role="status"
      aria-label={t("加载中…")}
      aria-busy="true"
    >
      {Array.from({ length: 3 }, (_, i) => (
        <div className="node-card skeleton-card" key={i}>
          <Skeleton />
          <Skeleton wide />
          <Skeleton />
          <Skeleton wide />
        </div>
      ))}
    </div>
  );
}

export default function ConsoleSkeleton() {
  return (
    <div className="shell console-skeleton" aria-busy="true">
      <aside className="sidebar" aria-hidden="true">
        <div className="brand">
          <img className="brand-logo" src="/xingdu-logo.png" alt="" />
          <span>
            {t("星渡")}
            <small>XINGDU</small>
          </span>
        </div>
        <div className="skeleton-workspace">
          <Skeleton wide />
        </div>
        <p className="nav-label">{t("工作台")}</p>
        <div className="skeleton-navigation">
          {Array.from({ length: 8 }, (_, i) => (
            <Skeleton wide key={i} />
          ))}
        </div>
      </aside>
      <div className="body">
        <main>
          <div className="skeleton-heading" aria-hidden="true">
            <Skeleton />
            <Skeleton wide />
          </div>
          <section className="panel">
            <TableSkeleton />
          </section>
        </main>
      </div>
    </div>
  );
}
