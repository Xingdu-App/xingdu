import APIDocsPage from "./APIDocsPage";
import { useMarketingText } from "./marketing-locale";
import "./DiscoveryPages.css";

const compatibility = [
  ["Shadowsocks / SS2022", "TCP", "0.8.0-dev", "✓", "✓", "✓", "—"],
  ["Trojan / VMess", "TCP + TLS", "0.7.0-dev", "✓", "✓", "✓", "✓"],
  ["VLESS", "TCP + TLS", "0.7.0-dev", "✓", "✓", "—", "✓"],
  ["Hysteria 2", "QUIC / UDP", "0.7.0-dev", "✓", "✓", "✓", "✓"],
  ["TUIC v5", "QUIC / UDP", "0.7.0-dev", "✓", "✓", "✓", "—"],
  ["AnyTLS / HTTPS", "TCP + TLS", "0.10.0-dev", "✓", "✓", "—", "—"],
];
const extension = [
  ["SOCKS5 / Mixed", "TCP", "0.14.0-dev", "✓", "✓", "✓", "—"],
  ["Hysteria 1", "QUIC / UDP", "0.14.0-dev", "✓", "✓", "—", "—"],
  ["ShadowTLS v3 + SS2022", "TCP", "0.14.0-dev", "✓", "✓", "—", "—"],
  ["Snell v4 compatible", "TCP", "0.14.0-dev", "✓", "—", "✓", "—"],
  ["Snell v6 beta", "TCP", "0.14.0-dev", "—", "—", "✓", "—"],
];
function Matrix({ rows }: { rows: string[][] }) {
  const t = useMarketingText();
  return (
    <div
      className="discovery-table"
      role="region"
      aria-label={t("协议与订阅格式对照")}
      tabIndex={0}
    >
      <table>
        <caption>
          {t(
            "✓ 表示已实现格式导出，不表示所有客户端版本已完成联网验收；— 表示暂不提供该格式。",
          )}
        </caption>
        <thead>
          <tr>
            {[
              "协议",
              "传输",
              "最低 Agent",
              "Stash",
              "Mihomo",
              "Surge",
              "Loon",
            ].map((x) => (
              <th scope="col" key={x}>
                {t(x)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row[0]}>
              {row.map((v, i) =>
                i === 0 ? (
                  <th scope="row" key={i}>
                    {v}
                  </th>
                ) : (
                  <td key={i}>{v}</td>
                ),
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
export default function DiscoveryPages({
  page,
}: {
  page: "personal-vps" | "protocols" | "api-docs";
}) {
  const t = useMarketingText();
  const title =
    page === "personal-vps"
      ? "一个人的 VPS，也值得有条理。"
      : page === "protocols"
        ? "协议与客户端兼容范围"
        : "星渡 API 文档";
  return (
    <article className="site-container discovery-page">
      <nav className="blog-breadcrumbs" aria-label={t("面包屑")}>
        <a href="/">{t("首页")}</a>
        <span aria-hidden="true">/</span>
        <span aria-current="page">{t(title)}</span>
      </nav>
      <header className="site-page-intro">
        <p className="site-eyebrow">
          XINGDU · {page === "personal-vps" ? "PERSONAL VPS" : "DOCUMENTATION"}
        </p>
        <h1>{t(title)}</h1>
        <p>
          {t(
            "星渡 Xingdu 是面向个人和小型团队的 VPS 与代理节点管理工具，提供服务器接入、协议部署、节点管理和客户端订阅。",
          )}
        </p>
      </header>
      {page === "api-docs" ? (
        <>
          <p>
            {t(
              "公开文档无需登录。执行接口仍需有效 API Key；以下云端示例使用 https://xingdu.app，自托管请替换为自己的控制端地址。",
            )}
          </p>
          <APIDocsPage publicView />
        </>
      ) : page === "personal-vps" ? (
        <div className="site-document">
          <section>
            <h2>{t("从一台开始，不必先组建团队")}</h2>
            <p>
              {t(
                "你可以独自使用一个工作空间，登记不同厂商的 VPS，查看 Agent 心跳并管理节点。需要协作时再邀请成员。星渡不提供 VPS 或公网线路，服务器、域名与流量由你自行准备。",
              )}
            </p>
            <p>
              <a className="site-text-link" href="/app">
                {t("开始管理我的 VPS")}
              </a>{" "}
              · <a href="/pricing">{t("查看价格与部署方式")}</a>
            </p>
          </section>
          <section>
            <h2>{t("一次走通接入、部署和订阅")}</h2>
            <ol>
              <li>
                {t(
                  "添加服务器：填写名称、地址和标签。台账创建成功后，还需接入 Agent 才能看到心跳。",
                )}
              </li>
              <li>
                {t(
                  "选择权限：只看状态用探针模式；需要安装节点时，明确授权托管模式。SSH 安装前核对主机指纹。",
                )}
              </li>
              <li>
                {t(
                  "部署节点：选择协议与端口，按要求提供证书。安装成功只说明任务完成，还需验证网络和客户端。",
                )}
              </li>
              <li>
                {t(
                  "创建订阅：选择对应客户端格式和节点，导入后测试实际流量。订阅链接包含访问能力，请按密码保管。",
                )}
              </li>
            </ol>
            <p>
              <a href="/blog/first-vps-checklist">
                {t("跟随第一台 VPS 操作清单")}
              </a>
            </p>
          </section>
          <section>
            <h2>{t("什么时候适合用星渡？")}</h2>
            <p>
              {t(
                "当你希望把散落的服务器资料、协议部署记录和订阅入口集中管理时，可以从一台非关键 VPS 试用。星渡也提供组织范围的 API Key，方便把已有脚本接入管理流程。",
              )}
            </p>
            <p>
              {t(
                "如果你需要完整的云资源采购、任意远程 Shell 或所有客户端自动兼容，应先核对服务范围。服务器系统维护、备份与公网故障恢复仍需保留独立方案。",
              )}
            </p>
          </section>
          <section>
            <h2>{t("个人使用常见问题")}</h2>
            <h3>{t("只有一台 VPS 可以用吗？")}</h3>
            <p>
              {t(
                "可以。工作空间用于隔离你的资料，不要求邀请成员。免费额度与付费套餐以价格页和控制台为准。",
              )}
            </p>
            <h3>{t("可以管理不同厂商的服务器吗？")}</h3>
            <p>
              {t(
                "符合 Linux、架构、systemd 和网络接入要求的服务器可以接入，不要求迁移到同一家云厂商。应先在自己的系统上完成安装和恢复验证。",
              )}
            </p>
            <h3>{t("连接流量经过星渡云端吗？")}</h3>
            <p>
              {t(
                "通常由客户端连接你自己的节点，控制端负责管理任务与配置。配置了中转时，流量经过你选择的入口和出口；云端套餐不包含 VPS 带宽。",
              )}
            </p>
          </section>
        </div>
      ) : (
        <div className="site-document">
          <section>
            <h2>{t("基础协议与导出格式")}</h2>
            <p>
              {t(
                "以下是版本能力说明，实际可创建的协议以你部署的控制端和 Agent 版本为准。服务器运行时与客户端适配器独立，能够生成配置不等于所有 App 版本都已通过联网验证。",
              )}
            </p>
            <Matrix rows={compatibility} />
          </section>
          <section>
            <h2>{t("开发中的扩展协议")}</h2>
            <p>
              {t(
                "以下扩展已完成本地实现与 Debian arm64 容器连接验证，尚未据此确认云端上线或真实客户端兼容；需要 Agent 0.14.0-dev。请等待对应版本发布后再使用。",
              )}
            </p>
            <Matrix rows={extension} />
          </section>
          <section>
            <h2>{t("选择前需要确认什么？")}</h2>
            <ul>
              <li>
                {t(
                  "TLS 协议需要匹配域名的有效证书。Stash、Mihomo 和 Surge 的相关导出使用证书固定；Loon 导出要求完整的系统信任证书链。",
                )}
              </li>
              <li>
                {t(
                  "Shadowsocks 系列当前仅 TCP。Hysteria 2 和 TUIC 使用 UDP 传输，需要放行相应 UDP 端口；QUIC 是传输方式。",
                )}
              </li>
              <li>
                {t(
                  "Mixed 同端口提供 HTTP 与 SOCKS5，订阅使用 SOCKS5。SOCKS5 / Mixed 不加密，应通过可信网络或加密隧道接入。",
                )}
              </li>
              <li>
                {t(
                  "ShadowTLS 使用 v3 与独立的 SS2022 内层凭据。Snell 兼容模式导出客户端 v4；v6 仍是测试版。Naive 暂不提供。",
                )}
              </li>
              <li>
                {t(
                  "Hysteria 2 分享链接仅支持 Hysteria 2，不包含完整分流规则。未列出的客户端不能由相似格式推定兼容。",
                )}
              </li>
            </ul>
          </section>
          <section>
            <h2>{t("怎样判断已经可用？")}</h2>
            <p>
              {t(
                "先确认 Agent 心跳，再确认服务启动和证书有效，最后在实际客户端测试访问与所需的 TCP / UDP 流量。容器客户端结果、App 导入结果和公网线路结果应分别记录。",
              )}
            </p>
            <p>
              <a href="/blog/client-subscription-guide">
                {t("阅读订阅配置指南")}
              </a>{" "}
              ·{" "}
              <a href="/blog/hysteria2-deployment-checklist">
                {t("阅读 Hysteria 2 部署清单")}
              </a>
            </p>
          </section>
        </div>
      )}
      <aside className="document-note">
        <p>
          <a href="/personal-vps">{t("个人 VPS 管理")}</a> ·{" "}
          <a href="/protocols">{t("协议兼容")}</a> ·{" "}
          <a href="/docs/api">API {t("文档")}</a> ·{" "}
          <a href="/security">{t("安全设计")}</a>
        </p>
        <p>
          {t(
            "遇到问题可联系 info@xingdu.app。请勿发送服务器密码、私钥或完整订阅链接。",
          )}
        </p>
      </aside>
    </article>
  );
}
