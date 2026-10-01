import type { Host } from "./api";
import { t } from "./i18n";

function memory(bytes: number) {
  return bytes >= 1024 ** 3
    ? `${(bytes / 1024 ** 3).toFixed(1)} GiB`
    : `${Math.round(bytes / 1024 ** 2)} MiB`;
}

export default function HostSystem({ host }: { host: Host }) {
  const m = host.metrics;
  if (!m || !m.os || m.cpus < 1)
    return <span className="host-system-empty">{t("暂无系统信息")}</span>;
  const os =
    (
      { linux: "Linux", darwin: "macOS", windows: "Windows" } as Record<
        string,
        string
      >
    )[m.os] ?? m.os;
  return (
    <div className="host-system">
      <span>
        {os} · {m.arch} · {t("{0} 核", { 0: m.cpus })}
      </span>
      <small>
        {t("内存")}{" "}
        {m.memory_total_bytes > 0
          ? `${memory(Math.max(0, m.memory_total_bytes - m.memory_available_bytes))} / ${memory(m.memory_total_bytes)}`
          : "—"}
      </small>
      <small>
        {t("负载（1 分钟）")}{" "}
        {Number.isFinite(m.load_1) ? m.load_1.toFixed(2) : "—"}
      </small>
      {host.status !== "online" && <small>{t("最后上报的数据")}</small>}
    </div>
  );
}
