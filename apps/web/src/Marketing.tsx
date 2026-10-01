import DiscoveryPages from "./DiscoveryPages";
import MarketingAnalytics from "./MarketingAnalytics";
import { paidPlans, enterpriseContact } from "./plans";
import { useContext, useEffect } from "react";
import { useLocale } from "./i18n";
import MarketingLanguage from "./MarketingLanguage";
import {
  MarketingLocale,
  useMarketingText,
  translateMarketing,
} from "./marketing-locale";
import { publicPages } from "./public-pages";
import Blog, { BlogCards } from "./Blog";
import { blogPosts } from "./blog-posts";
import type { ReactNode } from "react";
import "./Marketing.css";

import type { PublicPage } from "./public-pages";

const repository = "https://github.com/Xingdu-App/xingdu";
function useDocumentationLink() {
  const locale = useContext(MarketingLocale);
  return `${repository}/blob/main/${locale === "en" ? "README.md" : "README.zh-CN.md"}`;
}
function Arrow() {
  return <span aria-hidden="true">↗</span>;
}
function Brand() {
  const t = useMarketingText();
  return (
    <a className="site-brand" href="/" aria-label={t("星渡首页")}>
      <img src="/xingdu-logo.png" alt="" width="36" height="36" />
      <span>
        {t("星渡")}
        <small>XINGDU</small>
      </span>
    </a>
  );
}
function SectionTitle({
  eyebrow,
  title,
  children,
}: {
  eyebrow: string;
  title: string;
  children?: ReactNode;
}) {
  return (
    <div className="site-section-heading">
      <p className="site-eyebrow">{eyebrow}</p>
      <h2>{title}</h2>
      {children && <p>{children}</p>}
    </div>
  );
}
function ProductPreview() {
  const t = useMarketingText();
  return (
    <div className="product-stage" aria-label={t("控制台界面示意，非实时数据")}>
      <div className="preview-orbit orbit-one" aria-hidden="true" />
      <div className="preview-orbit orbit-two" aria-hidden="true" />
      <div className="preview-float">
        <span className="site-live-dot" /> {t(" 一个空间，管理你的 VPS")}
      </div>
      <div className="product-preview">
        <div className="preview-chrome">
          <span className="preview-window-dots" aria-hidden="true">
            ● ● ●
          </span>
          <span>{t("星渡 / 工作空间")}</span>
          <span className="preview-account">XD</span>
        </div>
        <div className="preview-inner">
          <aside className="preview-sidebar" aria-hidden="true">
            <img src="/xingdu-logo.png" alt="" width="28" height="28" />
            <span className="is-current">▤</span>
            <span>⌁</span>
            <span>⚙</span>
            <i />
          </aside>
          <div className="preview-content">
            <div className="preview-heading">
              <div>
                <small>YOUR WORKSPACE</small>
                <h3>{t("每一台，都在这里。")}</h3>
              </div>
              <span className="preview-pill">{t("我的空间")}</span>
            </div>
            <div className="preview-metrics">
              <div>
                <span>{t("已接入服务器")}</span>
                <strong>
                  03<small> {t(" 台")}</small>
                </strong>
              </div>
              <div>
                <span>{t("在线状态")}</span>
                <strong className="preview-green">{t("全部在线")}</strong>
              </div>
            </div>
            <div className="preview-machine">
              <span className="preview-machine-icon">U</span>
              <div>
                <strong>Ubuntu</strong>
                <small>{t("Agent · 探针模式")}</small>
              </div>
              <span className="preview-sparkline" aria-hidden="true">
                ▂ ▄ ▃ ▆ ▃ ▅ ▃
              </span>
              <span className="preview-status">{t("在线")}</span>
            </div>
            <div className="preview-machine">
              <span className="preview-machine-icon">D</span>
              <div>
                <strong>Debian</strong>
                <small>{t("SSH · 密码安装")}</small>
              </div>
              <span className="preview-sparkline" aria-hidden="true">
                ▃ ▂ ▄ ▃ ▅ ▄ ▂
              </span>
              <span className="preview-status">{t("在线")}</span>
            </div>
            <div className="preview-machine">
              <span className="preview-machine-icon">A</span>
              <div>
                <strong>Amazon Linux</strong>
                <small>{t("SSH · 私钥安装")}</small>
              </div>
              <span className="preview-sparkline" aria-hidden="true">
                ▂ ▄ ▂ ▃ ▅ ▃ ▄
              </span>
              <span className="preview-status">{t("在线")}</span>
            </div>
            <div className="preview-bottom">
              <span className="site-live-dot" />{" "}
              {t(" 机器状态 · 协议部署 · 客户端配置")}
            </div>
          </div>
        </div>
      </div>
      <p className="preview-caption">{t("产品界面示意 · 非实时数据")}</p>
    </div>
  );
}
function GettingStarted() {
  const t = useMarketingText();
  return (
    <section className="site-section site-container">
      <SectionTitle eyebrow="GET STARTED" title={t("三步，建立你的工作空间。")}>
        {t(
          "准备一台你有权管理的 Linux 服务器；使用 TLS 协议时，还需准备域名和有效证书。",
        )}
      </SectionTitle>
      <div className="site-feature-grid">
        {[
          [
            t("创建或加入组织"),
            t(
              "使用当前开放的方式登录，创建用于管理 VPS 的工作空间。个人使用也可以创建组织，无需邀请其他成员；需要协作时再邀请他人加入。",
            ),
          ],
          [
            t("添加服务器并接入"),
            t(
              "先登记服务器，再安装 Agent。只查看状态可使用探针模式；需要部署协议时，由管理员授权托管模式。",
            ),
          ],
          [
            t("部署节点并生成配置"),
            t(
              "提交前检查证书与端口，部署后查看节点状态，再创建客户端订阅。导入对应 App 后，验证实际连接。",
            ),
          ],
        ].map(([title, body], index) => (
          <article className="site-feature" key={title}>
            <span className="feature-number">0{index + 1}</span>
            <h3>{title}</h3>
            <p>{body}</p>
          </article>
        ))}
      </div>
      <a className="site-text-link" href="/help">
        {t("阅读入门与常见问题 ")}
        <Arrow />
      </a>
    </section>
  );
}
function Plans({ compact = false }: { compact?: boolean }) {
  const docs = useDocumentationLink();
  const t = useMarketingText();
  return (
    <>
      <p className="site-plans-free">
        {t(
          "Xingdu Cloud 已正式上线。付费套餐通过 Stripe 结算，将产生真实扣款；金额与计费周期请以结账页面为准。",
        )}
      </p>
      <div className="site-plans">
        {paidPlans.map((plan) => (
          <article
            className={
              plan.id === "premium" ? "site-plan plan-hosted" : "site-plan"
            }
            key={plan.id}
          >
            {plan.id === "premium" && (
              <span className="plan-ribbon">{t("更多服务器，更大空间")}</span>
            )}
            <span className="plan-type">XINGDU CLOUD</span>
            <h3>{plan.name}</h3>
            <p>
              {t(
                plan.id === "start"
                  ? "适合个人与小型工作空间。"
                  : "适合更多服务器与团队协作。",
              )}
            </p>
            <div className="plan-price">
              ${plan.monthly}
              <span>{t("/ 月")}</span>
            </div>
            <p>
              {t(
                plan.id === "start"
                  ? "或 $40/年，年付省 $20。"
                  : "或 $200/年，年付省 $40。",
              )}
            </p>
            <a
              className={
                plan.id === "premium"
                  ? "site-button site-button-dark"
                  : "site-button site-button-outline"
              }
              href={"/app/billing?plan=" + plan.id}
            >
              {t("查看套餐")} <Arrow />
            </a>
            <ul>
              <li>
                {t(
                  plan.id === "start"
                    ? "每个组织最多管理 10 台服务器"
                    : "每个组织最多管理 50 台服务器",
                )}
              </li>
              <li>{t("机器状态监控与协议部署")}</li>
              <li>{t("节点订阅与组织成员协作")}</li>
              <li>{t("API 密钥与自动化管理")}</li>
              {!compact && <li>{t("月付或年付自动续费，可取消下次续费")}</li>}
            </ul>
            <p className="plan-footnote">
              {t("价格以美元（USD）计。需自备 VPS，服务器和带宽费用另计。")}
            </p>
          </article>
        ))}
        <article className="site-plan">
          <span className="plan-type">CUSTOM</span>
          <h3>Enterprise</h3>
          <p>{t("适合更大规模与定制需求。")}</p>
          <div className="plan-price">{t("定制报价")}</div>
          <p>{t("需要更多服务器额度或定制部署？联系我们获取方案与报价。")}</p>
          <a
            className="site-button site-button-outline"
            href={enterpriseContact}
          >
            {t("联系客服")} <Arrow />
          </a>
          <ul>
            <li>{t("服务器额度按需求协商")}</li>
            <li>{t("部署与接入方案咨询")}</li>
            <li>{t("服务范围与支持方式单独约定")}</li>
          </ul>
          <p className="plan-footnote">
            {t("定制报价与交付范围以双方确认的方案为准。")}
          </p>
        </article>
      </div>
      <p className="site-plans-free">
        {t("免费管理 1 台自有服务器，或使用 MIT 开源版本自行部署。")}{" "}
        <a href={docs}>
          {t("自部署指南")} <Arrow />
        </a>
      </p>
    </>
  );
}

function Home() {
  const t = useMarketingText();
  return (
    <>
      <section className="site-hero site-container">
        <div className="hero-copy">
          <span className="site-release">
            <span className="site-live-dot" /> {t(" 正式上线 ")}
            <span>·</span> {t(" 从一台 VPS 开始")}
          </span>
          <h1>
            {t("分散的服务器，")}
            <br />
            <em>{t("在星渡相遇。")}</em>
          </h1>
          <p>
            {t(
              "集中管理你的 VPS、部署协议节点，并为常用客户端生成配置。支持个人使用与团队协作。",
            )}
          </p>
          <div className="hero-actions">
            <a className="site-button site-button-dark" href="/app">
              {t("进入控制台 ")}
              <Arrow />
            </a>
            <a className="site-text-link" href={repository}>
              {t("探索开源项目 ")}
              <Arrow />
            </a>
          </div>
          <div className="hero-principles">
            <span>{t("MIT 开源")}</span>
            <span>{t("自由自托管")}</span>
            <span>{t("凭据由你选择保存")}</span>
          </div>
        </div>
        <ProductPreview />
      </section>
      <div className="site-compatibility site-container">
        <p>{t("从熟悉的系统开始")}</p>
        <div>
          <span>Ubuntu</span>
          <span>Debian</span>
          <span>Amazon Linux</span>
        </div>
        <small>
          <a href={`${repository}/blob/main/docs/MACHINE-ACCESS.md`}>
            {t("查看系统要求与兼容范围")}
          </a>
        </small>
      </div>
      <section id="features" className="site-section site-container">
        <SectionTitle
          eyebrow="LESS FRICTION. MORE CLARITY."
          title={t("接入、部署、使用。一处完成。")}
        >
          {t("把重复的管理步骤，整理成清晰的工作流。")}
        </SectionTitle>
        <div className="site-feature-grid">
          <article className="site-feature">
            <span className="feature-number">01 / CONNECT</span>
            <div className="feature-glyph" aria-hidden="true">
              ⌁
            </div>
            <h3>{t("用适合你的方式接入")}</h3>
            <p>
              {t(
                "在机器上主动安装 Agent，或使用 SSH 密码、私钥完成安装。已有的服务器，进入同一个工作空间。",
              )}
            </p>
            <div className="feature-tags">
              <span>Agent</span>
              <span>{t("SSH 密码")}</span>
              <span>{t("PEM 私钥")}</span>
            </div>
          </article>
          <article className="site-feature">
            <span className="feature-number">02 / DEPLOY</span>
            <div className="feature-glyph" aria-hidden="true">
              ▥
            </div>
            <h3>{t("从机器，到协议服务")}</h3>
            <p>
              {t(
                "托管 Agent 可安装九种协议，包括 Shadowsocks、Trojan、Hysteria 2 与 AnyTLS。按协议准备配置，确认授权后查看任务进度。",
              )}
            </p>
            <div className="feature-tags">
              <span>{t("多协议部署")}</span>
              <span>TCP / QUIC</span>
              <span>{t("部署与卸载")}</span>
            </div>
          </article>
          <article className="site-feature">
            <span className="feature-number">03 / COLLABORATE</span>
            <div className="feature-glyph" aria-hidden="true">
              ▦
            </div>
            <h3>{t("个人使用，按需共享")}</h3>
            <p>
              {t(
                "为自己的 VPS 建立工作空间，集中管理服务器与节点。需要共同维护时，再邀请成员并分配权限，无需共享账号。",
              )}
            </p>
            <div className="feature-tags">
              <span>{t("组织空间")}</span>
              <span>{t("成员邀请")}</span>
              <span>{t("角色权限")}</span>
            </div>
          </article>
        </div>
      </section>
      <GettingStarted />
      <section className="site-security-band">
        <div className="site-container security-band-inner">
          <div>
            <p className="site-eyebrow">TRUST, BY DESIGN</p>
            <h2>
              {t("连接可以简单。")}
              <br />
              {t("权限，需要认真。")}
            </h2>
            <p>
              {t(
                "默认使用托管模式，确认授权后可部署协议。SSH 安装前校验主机身份，凭据默认临时加密使用，是否长期保存由你决定。",
              )}
            </p>
            <a className="site-text-link" href="/security">
              {t("了解安全设计 ")}
              <Arrow />
            </a>
          </div>
          <div className="security-points">
            <article>
              <span>01</span>
              <div>
                <h3>{t("按组织隔离")}</h3>
                <p>{t("成员权限校验与数据库行级隔离共同约束访问。")}</p>
              </div>
            </article>
            <article>
              <span>02</span>
              <div>
                <h3>{t("凭据有边界")}</h3>
                <p>
                  {t("SSH 凭据与 TLS 私钥不回显；连接凭据仅管理员主动查看。")}
                </p>
              </div>
            </article>
            <article>
              <span>03</span>
              <div>
                <h3>{t("访问可撤销")}</h3>
                <p>{t("每台机器使用独立身份，管理员可撤销后续接入。")}</p>
              </div>
            </article>
          </div>
        </div>
      </section>
      <section className="site-section site-container">
        <div className="section-with-link">
          <SectionTitle
            eyebrow="YOUR INFRASTRUCTURE. YOUR CHOICE."
            title={t("选择你的运行方式。")}
          >
            {t("Starter $5/月起，Premium $20/月，Enterprise 按需定制。")}
          </SectionTitle>
          <a className="site-text-link" href="/pricing">
            {t("查看价格说明 ")}
            <Arrow />
          </a>
        </div>
        <Plans compact />
      </section>
      <section className="site-roadmap site-container">
        <div>
          <span className="site-roadmap-label">{t("接下来")}</span>
          <h2>{t("从服务部署，到客户端配置。")}</h2>
          <p>
            {t(
              "已提供九种协议的部署、受限客户端导出、配置版本恢复和单层 TCP 中转。客户端版本与协议组合需分别验证；公网连接效果取决于实际网络。",
            )}
          </p>
        </div>
        <div className="roadmap-clients">
          <span>Stash</span>
          <span>Surge</span>
          <span>Loon</span>
          <span>Hysteria 2 URI</span>
          <small>{t("提供受限格式导出 · App 联网待验收")}</small>
        </div>
      </section>
      <section className="site-section site-container">
        <SectionTitle
          eyebrow="FROM THE JOURNAL"
          title={t("让每一次管理，都有方法可循。")}
        >
          {t("从个人 VPS 接入到多服务器管理，找到适合自己的使用方法。")}
        </SectionTitle>
        <BlogCards posts={blogPosts.slice(0, 2)} />
        <a className="site-text-link" href="/blog">
          {t("浏览全部文章 ")}
          <Arrow />
        </a>
      </section>
      <section className="site-final-cta site-container">
        <p className="site-eyebrow">A SIMPLE BEGINNING</p>
        <h2>{t("下一段连接，从这里开始。")}</h2>
        <p>{t("从自己的第一台 VPS 开始。")}</p>
        <a className="site-button site-button-dark" href="/app">
          {t("打开星渡控制台 ")}
          <Arrow />
        </a>
      </section>
    </>
  );
}
function Pricing() {
  const t = useMarketingText();
  return (
    <div className="site-container">
      <section className="site-page-intro">
        <p className="site-eyebrow">SIMPLE CHOICES</p>
        <h1>
          {t("自由部署，")}
          <em>{t("按需选择。")}</em>
        </h1>
        <p>
          {t(
            "Starter：10 台服务器，$5/月或 $40/年。Premium：50 台服务器，$20/月或 $200/年。Enterprise 联系客服定制。",
          )}
        </p>
      </section>
      <Plans />
      <section className="pricing-explanation">
        <span aria-hidden="true">✧</span>
        <div>
          <h2>{t("开源免费，不等于基础设施零成本。")}</h2>
          <p>
            {t(
              "自托管版本采用 MIT 许可证，不收取软件许可费。你需要自行提供服务器并负责部署、备份与维护。云端版的费用与额度不会改变已发布代码的 MIT 许可。",
            )}
          </p>
        </div>
      </section>
      <section className="site-section site-faq">
        <SectionTitle eyebrow="A FEW MORE THINGS" title={t("你可能还想知道")} />
        <details>
          <summary>{t("如何购买和取消云端套餐？")}</summary>
          <p>
            {t(
              "Xingdu Cloud 已正式上线，付费套餐通过 Stripe 结算并产生真实扣款。组织所有者可在「套餐与账单」选择套餐、查看账单和管理续费；请在付款前核对金额与计费周期。",
            )}
          </p>
        </details>
        <details>
          <summary>{t("免费版本有机器或成员数量限制吗？")}</summary>
          <p>
            {t(
              "Cloud 免费额度为 1 台服务器，Starter 为 10 台，Premium 为 50 台，按组织计算。节点部署、客户端订阅与成员也有独立配额，可在组织设置中查看；服务器额度不代表其他资源无限。自托管实例的额度由部署者配置。",
            )}
          </p>
        </details>
        <details>
          <summary>{t("价格包含 VPS 和流量吗？")}</summary>
          <p>
            {t(
              "价格仅包含星渡控制端的管理服务。自部署版与云端版都需要你自行提供 VPS，服务器、域名和网络流量费用另行承担。",
            )}
          </p>
        </details>
        <details>
          <summary>{t("开源代码可以用于商业项目吗？")}</summary>
          <p>
            {t(
              "已发布代码采用 MIT 许可证，允许商业使用，需遵守其版权与许可声明保留要求。具体以项目中的",
            )}{" "}
            <a href={`${repository}/blob/main/LICENSE`}>LICENSE</a>{" "}
            {t("为准；商标与托管服务不等同于代码许可。")}
          </p>
        </details>
        <details>
          <summary>{t("支持一键部署协议和客户端订阅吗？")}</summary>
          <p>
            {t(
              "托管 Agent 可部署 Trojan、VLESS、VMess、Hysteria 2、TUIC、Shadowsocks、SS2022、AnyTLS 和 HTTPS。支持范围内可导出客户端配置，配置更新与单层 TCP 中转需新版 Agent。自动证书需要运营者另行配置，实际客户端兼容范围以验收记录为准。",
            )}
          </p>
        </details>
      </section>
    </div>
  );
}
function Privacy() {
  const t = useMarketingText();
  const privacySections = [
    {
      title: t("这份说明适用于什么"),
      body: (
        <>
          <p>
            {t(
              "本页说明 Xingdu Cloud、官网及开源软件涉及的数据处理，更新于 2026 年 9 月 29 日。Cloud 的数据问题请联系 info@xingdu.app；自托管实例由各自部署者负责，部署位置、日志、备份和保留期限可能不同。",
            )}
          </p>
          <p>
            {t(
              "如需了解 Cloud 的数据处理地区、服务商、数据保留或删除安排，请联系 info@xingdu.app。自托管实例的邮件与第三方登录由部署者配置，数据请求也应联系该实例部署者。",
            )}
          </p>
        </>
      ),
    },
    {
      title: t("控制台处理的数据"),
      body: (
        <>
          <p>
            {t(
              "邮箱注册保存邮箱、验证时间和密码哈希；已有用户名账号仍可登录。通过 Google / GitHub 创建的账号保存该服务的用户标识与返回的已验证邮箱，不会获得你的第三方账号密码。验证完成前只保存限时注册挑战，不创建可登录账户。组织名称、成员关系、角色、邀请状态，以及你添加的服务器地址、SSH 用户、端口、标签和备注会保存在该实例的数据库中。",
            )}
          </p>
          <p>
            {t(
              "Agent 上报主机名、系统类型、架构、版本、运行时间、CPU 核数、内存与负载，用于显示机器状态。当前探针不采集文件内容、环境变量或进程列表。",
            )}
          </p>
        </>
      ),
    },
    {
      title: t("SSH 密码、私钥与机器身份"),
      body: (
        <>
          <p>
            {t(
              "主动安装 Agent 无需把 SSH 凭据交给控制端。选择 SSH 安装时，密码、私钥和私钥口令会由控制端处理，用于连接你指定的机器。",
            )}
          </p>
          <p>
            {t(
              "任务凭据以 AES-256-GCM 加密保存，任务完成、失败、取消或过期后清除任务中的密文；自动过期清理需要任务服务正常运行。你也可以明确选择长期加密保存，并在控制台删除长期副本。",
            )}
          </p>
          <p>
            {t(
              "加密密钥由部署者保管，控制端执行 SSH 安装时需要解密凭据。这是静态存储加密，不是对服务端不可见的端到端加密。每台 Agent 另有独立身份，数据库保存其摘要。",
            )}
          </p>
        </>
      ),
    },
    {
      title: t("协议证书与连接凭据"),
      body: (
        <>
          <p>
            {t(
              "创建协议部署时，控制端处理你提供的 TLS 证书与私钥，并生成客户端认证凭据。敏感配置采用 AES-256-GCM 加密保存，绑定所属组织、机器与部署，交付给授权的托管 Agent 用于运行服务。",
            )}
          </p>
          <p>
            {t(
              "部署列表不返回凭据。只有组织所有者或管理员主动查看连接信息时，才返回客户端认证凭据及公开证书，并记录审计；TLS 私钥不回显。机器上的服务配置仍需包含运行所需的密钥，并以受限权限保存。",
            )}
          </p>
          <p>
            {t(
              "当前生成的协议运行时配置禁用流量日志。机器心跳、部署任务结果与审计事件仍会保存；这不代替对实际操作系统、代理或云平台日志配置的检查。",
            )}
          </p>
        </>
      ),
    },
    {
      title: t("Cookie、访问日志与第三方"),
      body: (
        <>
          <p>
            {t(
              "控制台使用必要的 xingdu_session Cookie 维持登录，会话有效期为 24 小时，并设置 HttpOnly 与 SameSite 限制。是否使用 Secure 属性取决于部署配置；公网部署应使用 HTTPS。退出登录会撤销对应会话并清除 Cookie。",
            )}
          </p>
          <p>
            {t(
              "官网配置 Google Analytics 4 后，访问公开页面时会自动加载统计，使用 Cookie 和浏览器标识收集页面访问、访问时间、来源网站、浏览器与设备信息，以及控制台、价格和 GitHub 入口点击，用于了解访问情况并改进网站内容与体验。Google 会接收统计请求及 IP 地址等网络信息。我们在标签中关闭广告个性化和 Google signals，不主动上报账号、组织、服务器、SSH 凭据、节点密钥或订阅链接；控制台、登录、API 以及带未知查询参数或片段的页面不加载统计。公开页面仅允许已知 AI 来源的 utm_source 标记，查询参数在上报前移除，只记录固定的来源名称。你可以通过浏览器设置或拦截工具限制 Cookie 和统计请求。Web 服务器或部署平台仍可能记录访问日志。",
            )}{" "}
            {t("统计还记录公开内容入口、套餐选择及页面滚动深度，用于分析内容和购买意向；入口点击不代表注册或支付成功。")}{" "}
            <a
              href="https://policies.google.com/technologies/partner-sites"
              target="_blank"
              rel="noreferrer"
            >
              {t("了解 Google 如何处理合作伙伴网站的数据")}
            </a>
          </p>
          <p>
            {t(
              "Xingdu Cloud 使用 Resend 投递注册与找回密码验证码、安全通知、组织邀请和套餐通知。投递会向 Resend 提供收件邮箱及相应邮件内容，可能包含组织名称、邀请链接或套餐状态；不包含密码、SSH 凭据和节点私钥。验证码仅保存摘要。异步通知队列保存收件邮箱、通知内容与投递状态，投递服务运行时清理超过 30 天的记录；这不代表所有业务数据或服务商副本的保留期限。自托管实例由部署者配置邮件服务。",
            )}
          </p>
          <p>
            {t(
              "选择 Google 或 GitHub 登录、绑定时，浏览器会跳转至对应服务授权。第三方会处理该次访问和授权请求，星渡服务端会获取用于核验身份的用户标识与已验证邮箱。星渡不会请求访问代码仓库、联系人或云盘；用于身份核验的访问令牌不作为长期凭据保存。第三方提供的已验证邮箱与星渡账号的已验证邮箱一致时，会自动关联并登录已有账号，无需先手动绑定。仅用户名相同不会触发关联；已经关联的第三方身份仍以其稳定标识识别账号。",
            )}
          </p>
          <p>
            {t(
              "第三方授权流程使用短期安全状态与 Cookie 校验请求来源。解绑会移除星渡账号与第三方身份的关联，不会删除第三方账号，也不一定撤销第三方服务中的应用授权；你可以在对应服务的账号设置中撤销授权。至少需要保留一种星渡登录方式。",
            )}
          </p>
          <p>
            {t(
              "云端计费启用后，支付页面与账单管理由 Stripe 提供。星渡保存组织对应的 Stripe 客户、订阅标识、账期和状态，不保存完整银行卡号或安全码；银行卡信息直接交给 Stripe。星渡不会把机器地址、SSH 凭据或节点密钥发送给 Stripe。",
            )}
          </p>
          <p>
            {t(
              "启用社区规则集后，客户端会向规则托管服务请求下载和更新，托管方可接收到该请求的来源 IP 等网络信息。星渡不会为此主动发送你的客户端订阅链接或节点凭据。点击 GitHub 等外部链接后，适用对应网站的数据处理规则；请勿在公开 Issue 中提交凭据或私人服务器信息。",
            )}
          </p>
        </>
      ),
    },
    {
      title: t("你的控制方式与删除边界"),
      body: (
        <>
          <p>
            {t(
              "组织管理员可删除服务器资料、删除保存的 SSH 凭据、撤销邀请或机器接入，也可移除成员。普通成员可访问的数据取决于其组织角色。",
            )}
          </p>
          <p>
            {t(
              "删除保存的 SSH 凭据不取消已提交任务持有的临时副本；需要同时终止后续接入时，应撤销机器接入。撤销不会停止已部署的协议服务，也不会卸载机器上的软件；需要终止服务时，应先卸载并确认完成，再撤销接入。已发出的操作无法撤回。",
            )}
          </p>
          <p>
            {t(
              "在线数据删除不意味着备份中的历史副本立即消失，已记录的审计事件也可能继续保留。当前尚无账号和组织的一键删除功能。星渡相关数据咨询请通过下方邮箱联系，并说明请求类型和账号邮箱；请勿发送密码或密钥。自托管实例的数据查阅、删除与备份请求，请联系该实例部署者。",
            )}
          </p>
        </>
      ),
    },
  ];

  return (
    <div className="site-container">
      <section className="site-page-intro">
        <p className="site-eyebrow">PRIVACY, IN PLAIN WORDS</p>
        <h1>
          {t("数据的去向，")}
          <em>{t("应当清楚。")}</em>
        </h1>
        <p>{t("说明收集什么、用于什么，以及你可以控制什么。")}</p>
        <span className="document-version">
          {t("当前版本数据说明 · 更新于 2026.09.29")}
        </span>
      </section>
      <div className="site-document-layout">
        <nav aria-label={t("隐私说明目录")}>
          {privacySections.map((s, i) => (
            <a key={s.title} href={`#privacy-${i}`}>
              {String(i + 1).padStart(2, "0")}
              <span>{s.title}</span>
            </a>
          ))}
        </nav>
        <div className="site-document">
          {privacySections.map((s, i) => (
            <section key={s.title} id={`privacy-${i}`}>
              <h2>{s.title}</h2>
              {s.body}
            </section>
          ))}
          <div className="document-note">
            {t("隐私与数据咨询：")}
            <a href="mailto:info@xingdu.app">info@xingdu.app</a>
            {t("。想进一步了解权限和凭据保护？")}
            <a href="/security">
              {t("查看安全设计 ")}
              <Arrow />
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}
function Help() {
  const docs = useDocumentationLink();
  const t = useMarketingText();
  return (
    <div className="site-container">
      <section className="site-page-intro">
        <p className="site-eyebrow">HELP & SUPPORT</p>
        <h1>
          {t("从第一次登录，")}
          <em>{t("到第一台服务器。")}</em>
        </h1>
        <p>{t("了解使用步骤、权限分工，以及遇到问题时如何继续。")}</p>
      </section>
      <section className="site-faq">
        {[
          [
            t("只有自己使用、一台 VPS，也适合吗？"),
            t(
              "可以。星渡支持个人管理自己的 VPS，一台服务器也可以开始使用。注册时创建的组织就是资源工作空间，不要求有多名成员，也不需要邀请他人。以后需要共同维护时，再按需邀请成员。",
            ),
          ],
          [
            t("没有账号，如何开始？"),
            t(
              "在控制台使用已开放的登录方式。邮箱注册需要完成验证码验证；如果注册未开放，请联系当前服务管理员。第三方已验证邮箱与已有账号的已验证邮箱相同时，可直接登录并自动关联，无需先手动绑定。",
            ),
          ],
          [
            t("收到团队邀请后怎么做？"),
            t(
              "打开邀请链接并登录，核对组织名称后接受邀请。链接过期或已撤销时，请邀请人重新发送。加入后，在侧边栏组织菜单切换工作空间。",
            ),
          ],
          [
            t("服务器、节点和客户端订阅有什么区别？"),
            t(
              "服务器是你管理的机器；节点是部署在机器上的协议服务；客户端订阅把选中的节点和分流规则生成可导入的配置。客户端订阅不是星渡云服务的付费套餐。",
            ),
          ],
          [
            t("为什么有些操作不可用？"),
            t(
              "操作范围由组织角色决定。接入机器、部署协议和管理客户端订阅需要所有者或管理员权限。达到资源上限时，请先在设置中查看用量，再联系服务管理员。",
            ),
          ],
          [
            t("显示在线或部署成功，就能连接了吗？"),
            t(
              "在线表示 Agent 最近成功上报心跳；部署成功表示机器完成了对应任务。客户端连接还取决于域名解析、防火墙、端口、证书和 App 支持情况，需要实际导入并测试。",
            ),
          ],
          [
            t("客户端格式应该怎么选？"),
            t(
              "按实际使用的客户端选择，不能把一种格式直接当作另一种使用。Surge 不导出 VLESS，Loon 不导出 TUIC 且要求系统信任的完整证书链；Hysteria 2 分享链接仅用于该协议且不支持分流规则。Shadowrocket 暂无专用输出。",
            ),
          ],
          [
            t("没有收到验证码或忘记密码怎么办？"),
            t(
              "先检查垃圾邮件、邮箱拼写和重发倒计时，重发后请使用最新验证码。忘记密码时，在登录页选择「忘记密码」，通过邮箱验证码重置。无法访问邮箱时，可尝试已绑定的登录方式或联系支持。自托管实例需配置邮件服务；请勿向他人发送密码或验证码。",
            ),
          ],
          [
            t("删除订阅或撤销 Agent 会停止节点吗？"),
            t(
              "停用、删除或重置订阅链接会阻止旧链接继续获取配置，但不会撤回已下载的节点凭据。撤销 Agent 也不会停止机器上运行的服务。需要终止服务时，应先完成协议卸载并确认结果，再撤销接入。",
            ),
          ],
        ].map(([title, body]) => (
          <details key={title}>
            <summary>{title}</summary>
            <p>{body}</p>
          </details>
        ))}
      </section>
      <section className="security-boundary">
        <h2>{t("获取帮助与反馈")}</h2>
        <p>
          {t(
            "星渡使用与隐私问题可联系 info@xingdu.app。自托管实例的账号、组织权限、额度和数据处理问题，请联系当前实例的服务管理员。自托管部署步骤见部署文档；可公开的问题可提交到项目 Issue，附上操作步骤、发生时间和脱敏后的错误提示。",
          )}
        </p>
        <p>
          {t(
            "请勿公开密码、验证码、SSH 私钥、订阅链接或服务器地址。安全漏洞的报告要求见安全说明；当前没有已公布的客服响应时限。",
          )}
        </p>
        <p>
          <a href="mailto:info@xingdu.app">{t("联系支持：info@xingdu.app")}</a>
        </p>
        <p>
          <a href={docs}>{t("部署文档")}</a> ·{" "}
          <a href={`${repository}/issues`}>{t("问题反馈")}</a> ·{" "}
          <a href={`${repository}/blob/main/SECURITY.md`}>
            {t("安全报告说明")}
          </a>
        </p>
      </section>
    </div>
  );
}
function Service() {
  const t = useMarketingText();
  return (
    <div className="site-container">
      <section className="site-page-intro">
        <p className="site-eyebrow">SERVICE SCOPE</p>
        <h1>
          {t("开始使用前，")}
          <em>{t("了解服务范围。")}</em>
        </h1>
        <p>{t("了解 Xingdu Cloud 的功能、计费方式与使用边界。")}</p>
      </section>
      <div className="site-document">
        <section>
          <h2>{t("你在星渡管理什么")}</h2>
          <p>
            {t(
              "星渡提供服务器资料管理、Agent 接入、协议部署、客户端配置与组织协作。服务器、域名、证书及网络流量由你自行准备；星渡不提供 VPS，也不承诺特定网络环境下的连接效果。",
            )}
          </p>
        </section>
        <section>
          <h2>{t("按授权使用资源")}</h2>
          <p>
            {t(
              "只添加你拥有或已获授权管理的服务器。邀请成员前确认其职责，按需分配权限；不要分享个人账号。启用托管 Agent 前，请了解其安装、重启和卸载服务所需的权限。",
            )}
          </p>
        </section>
        <section>
          <h2>{t("功能范围与版本更新")}</h2>
          <p>
            {t(
              "功能与接口会随版本更新调整，当前不提供服务等级（SLA）承诺。自动证书和外部探测需要单独配置；可导出配置不代表所有客户端版本均已完成联网验证。升级自托管实例前，请备份并核对版本说明。",
            )}
          </p>
        </section>
        <section>
          <h2>{t("费用与资源上限")}</h2>
          <p>
            {t(
              "MIT 开源版本不收取软件许可费，自托管基础设施与运维由部署者承担。Cloud 套餐按组织计算，不包含 VPS 和带宽；各项资源上限以控制台为准。付费套餐通过 Stripe 结算，以美元计价并产生真实扣款。请核对结账页面的金额与计费周期，在「套餐与账单」管理订阅；税费、退款及账单问题请联系 info@xingdu.app。",
            )}
          </p>
        </section>
        <section>
          <h2>{t("停止使用与数据处理")}</h2>
          <p>
            {t(
              "停止使用前，请确认机器上的服务是否需要卸载，处理订阅链接和访问凭据，再撤销机器接入。当前未提供账号或组织的一键删除入口；数据查阅、删除及备份处理请联系实际部署者，处理边界见隐私说明。",
            )}
          </p>
        </section>
        <section>
          <h2>{t("文档与支持")}</h2>
          <p>
            {t(
              "使用问题请先查看帮助中心。自托管实例由各自部署者维护；可通过 info@xingdu.app 联系星渡。正式托管服务的运营主体、支持时间与数据处理政策尚待公布。",
            )}
          </p>
          <p>
            <a href="/help">{t("帮助中心")}</a> ·{" "}
            <a href="/privacy">{t("隐私说明")}</a> ·{" "}
            <a href="/pricing">{t("价格与部署方式")}</a>
          </p>
        </section>
      </div>
    </div>
  );
}
function Security() {
  const t = useMarketingText();
  return (
    <div className="site-container">
      <section className="site-page-intro">
        <p className="site-eyebrow">SMALLER ACCESS. CLEARER BOUNDARIES.</p>
        <h1>
          {t("需要的权限，")}
          <em>{t("清晰的边界。")}</em>
        </h1>
        <p>{t("安全来自每一步具体设计，而不是一句笼统的保证。")}</p>
      </section>
      <div className="security-design-grid">
        {[
          [
            t("组织访问隔离"),
            t(
              "组织数据受到成员角色校验与 PostgreSQL 行级安全策略约束。机器身份与浏览器登录身份独立，Agent 凭据不能代替用户账号访问控制台。",
            ),
          ],
          [
            t("先确认机器，再提交凭据"),
            t(
              "SSH 安装必须固定并校验主机指纹。用户应通过云控制台或已有可信连接独立核对指纹；自动扫描本身不证明机器身份。",
            ),
          ],
          [
            t("凭据按用途加密"),
            t(
              "SSH 凭据、TLS 私钥及协议认证凭据采用 AES-256-GCM 加密存储，上下文绑定组织、机器与用途。TLS 私钥不回显；客户端连接凭据仅组织管理员主动查看。",
            ),
          ],
          [
            t("默认托管，按需探针"),
            t(
              "默认选择托管模式，需明确授权 root 权限，可执行固定的协议安装与卸载任务，不开放任意远程命令。只需采集状态时可选择使用专用低权限用户和 systemd 限制的探针模式。",
            ),
          ],
          [
            t("控制连接范围"),
            t(
              "SSH 目标默认拒绝私有、回环、链路本地和已知云元数据地址。私有管理网段需由部署者单独允许，租户不能自行放宽。",
            ),
          ],
          [
            t("独立身份，可撤销"),
            t(
              "Agent 使用 HTTPS 和独立机器凭据主动连接控制端。管理员可撤销身份，阻止后续心跳和未完成注册；撤销不等同于远程卸载。",
            ),
          ],
        ].map(([title, body], i) => (
          <article key={title}>
            <span className="feature-number">0{i + 1}</span>
            <h2>{title}</h2>
            <p>{body}</p>
          </article>
        ))}
      </div>
      <section className="security-boundary">
        <h2>{t("当前的验收范围")}</h2>
        <p>
          {t(
            "Ubuntu 24.04、Debian 13 和 Amazon Linux 2023 已完成 arm64 Docker 环境中的 systemd 安装、SSH 接入、心跳、重启、断联恢复与撤销验证。真实 VPS、EC2 和 amd64 运行验收仍需单独完成。",
          )}
        </p>
        <p>
          {t(
            "当前没有独立安全认证或 SLA 承诺。已提供加密备份与隔离恢复工具；实际恢复能力取决于部署配置与恢复演练。安全问题请通过公开联系渠道报告。",
          )}
        </p>
        <a
          className="site-text-link"
          href={`${repository}/blob/main/docs/MACHINE-ACCESS.md`}
        >
          {t("阅读完整的安全边界 ")}
          <Arrow />
        </a>
      </section>
    </div>
  );
}
export default function Marketing({ page }: { page: PublicPage }) {
  const locale = useLocale();
  useEffect(() => {
    document.documentElement.lang = locale;
    const info = publicPages[page];
    const article = info.articleSlug
      ? blogPosts.find((post) => post.slug === info.articleSlug)
      : undefined;
    const title =
      locale === "en" && article
        ? `${translateMarketing(article.title, locale)} · Xingdu Blog`
        : translateMarketing(info.title, locale);
    const description = translateMarketing(info.description, locale);
    document.title = title;
    for (const selector of [
      'meta[name="description"]',
      'meta[property="og:description"]',
      'meta[name="twitter:description"]',
    ]) {
      document.querySelector(selector)?.setAttribute("content", description);
    }
    for (const selector of [
      'meta[property="og:title"]',
      'meta[name="twitter:title"]',
    ]) {
      document.querySelector(selector)?.setAttribute("content", title);
    }
  }, [locale, page]);
  return (
    <MarketingLocale value={locale}>
      <MarketingContent page={page} />
    </MarketingLocale>
  );
}
function MarketingContent({ page }: { page: PublicPage }) {
  const docs = useDocumentationLink();
  const t = useMarketingText();
  return (
    <div className="marketing-site">
      <a className="site-skip" href="#site-content">
        {t("跳至主要内容")}
      </a>
      <header className="site-header">
        <div className="site-container site-nav">
          <Brand />
          <nav aria-label={t("官网导航")}>
            <a href="/#features">{t("产品")}</a>
            <a
              href="/pricing"
              aria-current={page === "pricing" ? "page" : undefined}
            >
              {t("价格")}
            </a>
            <a
              href="/security"
              aria-current={page === "security" ? "page" : undefined}
            >
              {t("安全")}
            </a>
            <a
              href="/privacy"
              aria-current={page === "privacy" ? "page" : undefined}
            >
              {t("隐私")}
            </a>
            <a
              href="/blog"
              aria-current={
                page === "blog" || page.startsWith("blog/") ? "page" : undefined
              }
            >
              {t("博客")}
            </a>
            <a href="/help" aria-current={page === "help" ? "page" : undefined}>
              {t("帮助")}
            </a>
          </nav>
          <div className="site-header-actions">
            <MarketingLanguage />
            <a className="site-login" href="/app">
              {t("登录控制台 ")}
              <Arrow />
            </a>
          </div>
        </div>
      </header>
      <main id="site-content" className="site-main">
        {page === "personal-vps" ||
        page === "protocols" ||
        page === "api-docs" ? (
          <DiscoveryPages page={page} />
        ) : page === "blog" || page.startsWith("blog/") ? (
          <Blog slug={page === "blog" ? undefined : page.slice(5)} />
        ) : page === "home" ? (
          <Home />
        ) : page === "pricing" ? (
          <Pricing />
        ) : page === "privacy" ? (
          <Privacy />
        ) : page === "help" ? (
          <Help />
        ) : page === "service" ? (
          <Service />
        ) : (
          <Security />
        )}
      </main>
      <footer className="site-footer">
        <div className="site-container">
          <div className="site-footer-top">
            <div>
              <Brand />
              <p>{t("你的 VPS 与节点，一处管理。")}</p>
            </div>
            <div className="site-footer-links">
              <div>
                <strong>{t("星渡")}</strong>
                <a href="/#features">{t("产品能力")}</a>
                <a href="/pricing">{t("价格与部署")}</a>
                <a href="/app">{t("控制台")}</a>
              </div>
              <div>
                <strong>{t("了解更多")}</strong>
                <a href="/personal-vps">{t("个人 VPS 管理")}</a>
                <a href="/protocols">{t("协议兼容")}</a>
                <a href="/docs/api">API {t("文档")}</a>
                <a href="/blog">{t("博客与指南")}</a>
                <a href="/help">{t("帮助中心")}</a>
                <a href="mailto:info@xingdu.app">{t("联系我们")}</a>
                <a href="/service">{t("服务范围")}</a>
                <a href="/privacy">{t("隐私说明")}</a>
                <a href="/security">{t("安全设计")}</a>
                <a href={docs}>{t("部署文档")}</a>
              </div>
              <div>
                <strong>{t("开放生态")}</strong>
                <a href={repository}>
                  GitHub <Arrow />
                </a>
                <a href={`${repository}/releases`}>
                  {t("版本动态 ")}
                  <Arrow />
                </a>
                <a href={`${repository}/blob/main/LICENSE`}>
                  {t("MIT 许可证 ")}
                  <Arrow />
                </a>
              </div>
            </div>
          </div>
          <div className="site-footer-bottom">
            <span>{t("© 2026 星渡 Xingdu")}</span>
            <span>{t("MIT 开源 · 云端服务")}</span>
            <span>Built to connect.</span>
            <MarketingAnalytics />
          </div>
        </div>
      </footer>
    </div>
  );
}
