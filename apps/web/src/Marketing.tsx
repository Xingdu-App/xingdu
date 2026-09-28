import type { ReactNode } from "react";
import "./Marketing.css";

import type { PublicPage } from "./public-pages";

const repository = "https://github.com/Xingdu-App/xingdu";
const docs = `${repository}/blob/main/README.zh-CN.md`;
function Arrow() {
  return <span aria-hidden="true">↗</span>;
}
function Brand() {
  return (
    <a className="site-brand" href="/" aria-label="星渡首页">
      <img src="/xingdu-logo.png" alt="" width="36" height="36" />
      <span>
        星渡<small>XINGDU</small>
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
  return (
    <div className="product-stage" aria-label="控制台界面示意，非实时数据">
      <div className="preview-orbit orbit-one" aria-hidden="true" />
      <div className="preview-orbit orbit-two" aria-hidden="true" />
      <div className="preview-float">
        <span className="site-live-dot" /> 一个空间，连接你的团队
      </div>
      <div className="product-preview">
        <div className="preview-chrome">
          <span className="preview-window-dots" aria-hidden="true">
            ● ● ●
          </span>
          <span>星渡 / 工作空间</span>
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
                <h3>每一台，都在这里。</h3>
              </div>
              <span className="preview-pill">团队空间</span>
            </div>
            <div className="preview-metrics">
              <div>
                <span>已接入服务器</span>
                <strong>
                  03<small> 台</small>
                </strong>
              </div>
              <div>
                <span>在线状态</span>
                <strong className="preview-green">全部在线</strong>
              </div>
            </div>
            <div className="preview-machine">
              <span className="preview-machine-icon">U</span>
              <div>
                <strong>Ubuntu</strong>
                <small>Agent · 探针模式</small>
              </div>
              <span className="preview-sparkline" aria-hidden="true">
                ▂ ▄ ▃ ▆ ▃ ▅ ▃
              </span>
              <span className="preview-status">在线</span>
            </div>
            <div className="preview-machine">
              <span className="preview-machine-icon">D</span>
              <div>
                <strong>Debian</strong>
                <small>SSH · 密码安装</small>
              </div>
              <span className="preview-sparkline" aria-hidden="true">
                ▃ ▂ ▄ ▃ ▅ ▄ ▂
              </span>
              <span className="preview-status">在线</span>
            </div>
            <div className="preview-machine">
              <span className="preview-machine-icon">A</span>
              <div>
                <strong>Amazon Linux</strong>
                <small>SSH · 私钥安装</small>
              </div>
              <span className="preview-sparkline" aria-hidden="true">
                ▂ ▄ ▂ ▃ ▅ ▃ ▄
              </span>
              <span className="preview-status">在线</span>
            </div>
            <div className="preview-bottom">
              <span className="site-live-dot" /> 组织协作 · 权限清晰 · 状态可见
            </div>
          </div>
        </div>
      </div>
      <p className="preview-caption">产品界面示意 · 非实时数据</p>
    </div>
  );
}
function Plans({ compact = false }: { compact?: boolean }) {
  return (
    <div className="site-plans">
      <article className="site-plan">
        <span className="plan-type">SELF-HOSTED</span>
        <h3>自己部署，自由掌握。</h3>
        <p>适合希望自主运行和管理数据的个人与团队。</p>
        <div className="plan-price">
          免费<span>MIT 开源版本</span>
        </div>
        <a className="site-button site-button-outline" href={docs}>
          开始自托管 <Arrow />
        </a>
        <ul>
          <li>服务器资料与 Agent 接入</li>
          <li>运行状态、心跳与离线判断</li>
          <li>组织、成员与访问权限</li>
          <li>SSH 安装与可选凭据加密保存</li>
          {!compact && <li>源代码可查看、修改和分发</li>}
        </ul>
        <p className="plan-footnote">
          软件免费；服务器、域名、存储与运维成本由你承担。
        </p>
      </article>
      <article className="site-plan plan-hosted">
        <span className="plan-ribbon">筹备中</span>
        <span className="plan-type">XINGDU CLOUD</span>
        <h3>更少维护，更多专注。</h3>
        <p>为希望减少控制端部署维护的团队准备。</p>
        <div className="plan-price price-pending">
          价格待公布<span>托管服务尚未开放</span>
        </div>
        <a
          className="site-button site-button-dark"
          href={`${repository}/releases`}
        >
          关注版本动态 <Arrow />
        </a>
        <ul>
          <li>计划由星渡运行和维护控制端</li>
          <li>沿用熟悉的组织协作体验</li>
          <li>套餐、额度与服务范围待公布</li>
          <li>开放前说明数据处理与计费规则</li>
          {!compact && <li>当前不提供购买或自动续费</li>}
        </ul>
        <p className="plan-footnote">
          本页不构成已上线的托管服务、价格或服务等级承诺。
        </p>
      </article>
    </div>
  );
}
function Home() {
  return (
    <>
      <section className="site-hero site-container">
        <div className="hero-copy">
          <span className="site-release">
            <span className="site-live-dot" /> 开源开发预览 <span>·</span>{" "}
            为协作而生
          </span>
          <h1>
            分散的服务器，
            <br />
            <em>在星渡相遇。</em>
          </h1>
          <p>
            一个清晰的工作空间，连接你的服务器与团队。掌握机器状态，分配访问权限，让每一次协作都有条不紊。
          </p>
          <div className="hero-actions">
            <a className="site-button site-button-dark" href="/app">
              进入控制台 <Arrow />
            </a>
            <a className="site-text-link" href={repository}>
              探索开源项目 <Arrow />
            </a>
          </div>
          <div className="hero-principles">
            <span>MIT 开源</span>
            <span>自由自托管</span>
            <span>凭据由你选择保存</span>
          </div>
        </div>
        <ProductPreview />
      </section>
      <div className="site-compatibility site-container">
        <p>从熟悉的系统开始</p>
        <div>
          <span>Ubuntu</span>
          <span>Debian</span>
          <span>Amazon Linux</span>
        </div>
        <small>已验证 arm64 Docker 环境中的 Agent 接入</small>
      </div>
      <section id="features" className="site-section site-container">
        <SectionTitle
          eyebrow="LESS FRICTION. MORE CLARITY."
          title="接入、部署、协作。一处完成。"
        >
          把重复的管理步骤，整理成清晰的工作流。
        </SectionTitle>
        <div className="site-feature-grid">
          <article className="site-feature">
            <span className="feature-number">01 / CONNECT</span>
            <div className="feature-glyph" aria-hidden="true">
              ⌁
            </div>
            <h3>用适合你的方式接入</h3>
            <p>
              在机器上主动安装 Agent，或使用 SSH
              密码、私钥完成安装。已有的服务器，进入同一个工作空间。
            </p>
            <div className="feature-tags">
              <span>Agent</span>
              <span>SSH 密码</span>
              <span>PEM 私钥</span>
            </div>
          </article>
          <article className="site-feature">
            <span className="feature-number">02 / DEPLOY</span>
            <div className="feature-glyph" aria-hidden="true">
              ▥
            </div>
            <h3>从机器，到协议服务</h3>
            <p>
              托管 Agent 可安装 Trojan、VLESS、VMess、Hysteria 2 与 TUIC。提供
              TLS 证书，确认授权后查看任务进度。
            </p>
            <div className="feature-tags">
              <span>TLS 加密</span>
              <span>TCP / QUIC</span>
              <span>部署与卸载</span>
            </div>
          </article>
          <article className="site-feature">
            <span className="feature-number">03 / COLLABORATE</span>
            <div className="feature-glyph" aria-hidden="true">
              ▦
            </div>
            <h3>共享工作，不共享账号</h3>
            <p>
              通过组织邀请团队成员，按角色分配权限。不同组织的服务器、凭据与成员访问分别管理。
            </p>
            <div className="feature-tags">
              <span>组织空间</span>
              <span>成员邀请</span>
              <span>角色权限</span>
            </div>
          </article>
        </div>
      </section>
      <section className="site-security-band">
        <div className="site-container security-band-inner">
          <div>
            <p className="site-eyebrow">TRUST, BY DESIGN</p>
            <h2>
              连接可以简单。
              <br />
              权限，需要认真。
            </h2>
            <p>
              从低权限探针开始。SSH
              安装前校验主机身份，凭据默认临时加密使用，是否长期保存由你决定。
            </p>
            <a className="site-text-link" href="/security">
              了解安全设计 <Arrow />
            </a>
          </div>
          <div className="security-points">
            <article>
              <span>01</span>
              <div>
                <h3>按组织隔离</h3>
                <p>成员权限校验与数据库行级隔离共同约束访问。</p>
              </div>
            </article>
            <article>
              <span>02</span>
              <div>
                <h3>凭据有边界</h3>
                <p>SSH 凭据与 TLS 私钥不回显；连接凭据仅管理员主动查看。</p>
              </div>
            </article>
            <article>
              <span>03</span>
              <div>
                <h3>访问可撤销</h3>
                <p>每台机器使用独立身份，管理员可撤销后续接入。</p>
              </div>
            </article>
          </div>
        </div>
      </section>
      <section className="site-section site-container">
        <div className="section-with-link">
          <SectionTitle
            eyebrow="YOUR INFRASTRUCTURE. YOUR CHOICE."
            title="选择你的运行方式。"
          >
            开源版本免费使用，托管服务正在准备中。
          </SectionTitle>
          <a className="site-text-link" href="/pricing">
            查看价格说明 <Arrow />
          </a>
        </div>
        <Plans compact />
      </section>
      <section className="site-roadmap site-container">
        <div>
          <span className="site-roadmap-label">接下来</span>
          <h2>从机器接入，走向线路编排。</h2>
          <p>
            五种协议的安装与卸载已实现，线路编排与多客户端订阅仍在计划中。部署完成后，需自行验证实际客户端连通性。
          </p>
        </div>
        <div className="roadmap-clients">
          <span>Stash</span>
          <span>Surge</span>
          <span>Loon</span>
          <span>Shadowrocket</span>
          <small>计划兼容 · 非当前功能</small>
        </div>
      </section>
      <section className="site-final-cta site-container">
        <p className="site-eyebrow">A SIMPLE BEGINNING</p>
        <h2>下一段连接，从这里开始。</h2>
        <p>从第一台服务器，或第一个协作空间开始。</p>
        <a className="site-button site-button-dark" href="/app">
          打开星渡控制台 <Arrow />
        </a>
      </section>
    </>
  );
}
function Pricing() {
  return (
    <div className="site-container">
      <section className="site-page-intro">
        <p className="site-eyebrow">SIMPLE CHOICES</p>
        <h1>
          自由部署，<em>按需选择。</em>
        </h1>
        <p>
          先找到适合你的运行方式。开源版本免费，托管服务价格将在开放前公布。
        </p>
      </section>
      <Plans />
      <section className="pricing-explanation">
        <span aria-hidden="true">✧</span>
        <div>
          <h2>开源免费，不等于基础设施零成本。</h2>
          <p>
            自托管版本采用 MIT
            许可证，不收取软件许可费。你需要自行提供服务器并负责部署、备份与维护。未来托管版的费用与额度不会改变已发布代码的
            MIT 许可。
          </p>
        </div>
      </section>
      <section className="site-section site-faq">
        <SectionTitle eyebrow="A FEW MORE THINGS" title="你可能还想知道" />
        <details>
          <summary>现在可以购买托管服务吗？</summary>
          <p>
            还不可以。目前没有已发布的托管套餐、在线支付或自动续费。本地控制台供部署者和已获授权的成员使用。
          </p>
        </details>
        <details>
          <summary>免费版本有机器或成员数量限制吗？</summary>
          <p>
            当前自托管版本尚未实现计费配额，实际容量取决于你的部署资源。这不是对未来托管服务额度的承诺。
          </p>
        </details>
        <details>
          <summary>价格包含 VPS 和流量吗？</summary>
          <p>
            自托管版本不包含
            VPS、域名、存储或网络流量。未来托管服务的包含项将在套餐发布时列明，目前没有相关收费承诺。
          </p>
        </details>
        <details>
          <summary>开源代码可以用于商业项目吗？</summary>
          <p>
            已发布代码采用 MIT
            许可证，允许商业使用，需遵守其版权与许可声明保留要求。具体以项目中的{" "}
            <a href={`${repository}/blob/main/LICENSE`}>LICENSE</a>{" "}
            为准；商标与托管服务不等同于代码许可。
          </p>
        </details>
        <details>
          <summary>支持一键部署协议和客户端订阅吗？</summary>
          <p>
            已实现托管 Agent 安装与卸载 Trojan、VLESS、VMess、Hysteria 2、TUIC
            v5，须提供 TLS
            证书并明确授权。线路编排与客户端订阅尚未开放，也不代表托管套餐已上线。
          </p>
        </details>
      </section>
    </div>
  );
}
const privacySections = [
  {
    title: "这份说明适用于什么",
    body: (
      <>
        <p>
          本页说明星渡当前开源版本与官网页面的数据处理方式，更新于 2026 年 9 月
          28
          日。不同自托管实例由各自部署者运行；部署位置、日志、备份和保留期限由实际部署配置决定。
        </p>
        <p>
          星渡托管服务尚未开放。运营主体、处理地区、服务商、保留期限及隐私联络方式将在服务开放前单独公布。本页不替代未来托管服务的正式隐私政策。
        </p>
      </>
    ),
  },
  {
    title: "控制台处理的数据",
    body: (
      <>
        <p>
          账号使用用户名和密码登录，服务器保存密码哈希而非明文密码。组织名称、成员关系、角色、邀请状态，以及你添加的服务器地址、SSH
          用户、端口、标签和备注会保存在该实例的数据库中。
        </p>
        <p>
          Agent 上报主机名、系统类型、架构、版本、运行时间、CPU
          核数、内存与负载，用于显示机器状态。当前探针不采集文件内容、环境变量或进程列表。
        </p>
      </>
    ),
  },
  {
    title: "SSH 密码、私钥与机器身份",
    body: (
      <>
        <p>
          主动安装 Agent 无需把 SSH 凭据交给控制端。选择 SSH
          安装时，密码、私钥和私钥口令会由控制端处理，用于连接你指定的机器。
        </p>
        <p>
          任务凭据以 AES-256-GCM
          加密保存，任务完成、失败、取消或过期后清除任务中的密文；自动过期清理需要任务服务正常运行。你也可以明确选择长期加密保存，并在控制台删除长期副本。
        </p>
        <p>
          加密密钥由部署者保管，控制端执行 SSH
          安装时需要解密凭据。这是静态存储加密，不是对服务端不可见的端到端加密。每台
          Agent 另有独立身份，数据库保存其摘要。
        </p>
      </>
    ),
  },
  {
    title: "协议证书与连接凭据",
    body: (
      <>
        <p>
          创建协议部署时，控制端处理你提供的 TLS
          证书与私钥，并生成客户端认证凭据。敏感配置采用 AES-256-GCM
          加密保存，绑定所属组织、机器与部署，交付给授权的托管 Agent
          用于运行服务。
        </p>
        <p>
          部署列表不返回凭据。只有组织所有者或管理员主动查看连接信息时，才返回客户端认证凭据及公开证书，并记录审计；TLS
          私钥不回显。机器上的服务配置仍需包含运行所需的密钥，并以受限权限保存。
        </p>
        <p>
          当前生成的协议运行时配置禁用流量日志。机器心跳、部署任务结果与审计事件仍会保存；这不代替对实际操作系统、代理或云平台日志配置的检查。
        </p>
      </>
    ),
  },
  {
    title: "Cookie、访问日志与第三方",
    body: (
      <>
        <p>
          控制台使用必要的 xingdu_session Cookie 维持登录，会话有效期为 24
          小时，并设置 HttpOnly 与 SameSite 限制。是否使用 Secure
          属性取决于部署配置；公网部署应使用 HTTPS。退出登录会撤销对应会话并清除
          Cookie。
        </p>
        <p>
          当前官网未接入广告或第三方行为分析脚本，也不加载第三方字体。Web
          服务器或部署平台可能记录 IP
          地址、访问时间、浏览器信息与请求路径，具体以部署配置为准。
        </p>
        <p>
          点击 GitHub 等外部链接后，将适用对应网站的数据处理规则。请不要把 SSH
          凭据、邀请令牌或私人服务器信息提交到公开 issue。
        </p>
      </>
    ),
  },
  {
    title: "你的控制方式与删除边界",
    body: (
      <>
        <p>
          组织管理员可删除服务器资料、删除保存的 SSH
          凭据、撤销邀请或机器接入，也可移除成员。普通成员可访问的数据取决于其组织角色。
        </p>
        <p>
          删除保存的 SSH
          凭据不取消已提交任务持有的临时副本；需要同时终止后续接入时，应撤销机器接入。撤销不会停止已部署的协议服务，也不会卸载机器上的软件；需要终止服务时，应先卸载并确认完成，再撤销接入。已发出的操作无法撤回。
        </p>
        <p>
          在线数据删除不意味着备份中的历史副本立即消失，已记录的审计事件也可能继续保留。当前尚无账号和组织的一键删除功能。自托管实例的数据查阅、删除与备份请求，请联系该实例部署者。
        </p>
      </>
    ),
  },
];
function Privacy() {
  return (
    <div className="site-container">
      <section className="site-page-intro">
        <p className="site-eyebrow">PRIVACY, IN PLAIN WORDS</p>
        <h1>
          数据的去向，<em>应当清楚。</em>
        </h1>
        <p>说明收集什么、用于什么，以及你可以控制什么。</p>
        <span className="document-version">
          当前版本数据说明 · 更新于 2026.09.28
        </span>
      </section>
      <div className="site-document-layout">
        <nav aria-label="隐私说明目录">
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
            想进一步了解权限和凭据保护？
            <a href="/security">
              查看安全设计 <Arrow />
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}
function Security() {
  return (
    <div className="site-container">
      <section className="site-page-intro">
        <p className="site-eyebrow">SMALLER ACCESS. CLEARER BOUNDARIES.</p>
        <h1>
          需要的权限，<em>清晰的边界。</em>
        </h1>
        <p>安全来自每一步具体设计，而不是一句笼统的保证。</p>
      </section>
      <div className="security-design-grid">
        {[
          [
            "组织访问隔离",
            "组织数据受到成员角色校验与 PostgreSQL 行级安全策略约束。机器身份与浏览器登录身份独立，Agent 凭据不能代替用户账号访问控制台。",
          ],
          [
            "先确认机器，再提交凭据",
            "SSH 安装必须固定并校验主机指纹。用户应通过云控制台或已有可信连接独立核对指纹；自动扫描本身不证明机器身份。",
          ],
          [
            "凭据按用途加密",
            "SSH 凭据、TLS 私钥及协议认证凭据采用 AES-256-GCM 加密存储，上下文绑定组织、机器与用途。TLS 私钥不回显；客户端连接凭据仅组织管理员主动查看。",
          ],
          [
            "从低权限探针开始",
            "默认探针使用专用低权限用户和 systemd 限制。选择托管模式需明确授权 root 权限，可执行固定的协议安装与卸载任务，不开放任意远程命令。",
          ],
          [
            "控制连接范围",
            "SSH 目标默认拒绝私有、回环、链路本地和已知云元数据地址。私有管理网段需由部署者单独允许，租户不能自行放宽。",
          ],
          [
            "独立身份，可撤销",
            "Agent 使用 HTTPS 和独立机器凭据主动连接控制端。管理员可撤销身份，阻止后续心跳和未完成注册；撤销不等同于远程卸载。",
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
        <h2>当前的验收范围</h2>
        <p>
          Ubuntu 24.04、Debian 13 和 Amazon Linux 2023 已完成 arm64 Docker
          环境中的 systemd 安装、SSH 接入、心跳、重启、断联恢复与撤销验证。真实
          VPS、EC2 和 amd64 运行验收仍需单独完成。
        </p>
        <p>
          当前是开发预览，尚未完成公网生产运营准备，也没有独立安全认证或 SLA
          承诺。加密密钥轮换、备份恢复、正式运营隐私政策等工作仍需在服务开放前完善。
        </p>
        <a
          className="site-text-link"
          href={`${repository}/blob/main/docs/MACHINE-ACCESS.md`}
        >
          阅读完整的安全边界 <Arrow />
        </a>
      </section>
    </div>
  );
}
export default function Marketing({ page }: { page: PublicPage }) {
  return (
    <div className="marketing-site">
      <a className="site-skip" href="#site-content">
        跳至主要内容
      </a>
      <header className="site-header">
        <div className="site-container site-nav">
          <Brand />
          <nav aria-label="官网导航">
            <a href="/#features">产品</a>
            <a
              href="/pricing"
              aria-current={page === "pricing" ? "page" : undefined}
            >
              价格
            </a>
            <a
              href="/security"
              aria-current={page === "security" ? "page" : undefined}
            >
              安全
            </a>
            <a
              href="/privacy"
              aria-current={page === "privacy" ? "page" : undefined}
            >
              隐私
            </a>
          </nav>
          <a className="site-login" href="/app">
            登录控制台 <Arrow />
          </a>
        </div>
      </header>
      <main id="site-content" className="site-main">
        {page === "home" ? (
          <Home />
        ) : page === "pricing" ? (
          <Pricing />
        ) : page === "privacy" ? (
          <Privacy />
        ) : (
          <Security />
        )}
      </main>
      <footer className="site-footer">
        <div className="site-container">
          <div className="site-footer-top">
            <div>
              <Brand />
              <p>连点成网，一键抵达。</p>
            </div>
            <div className="site-footer-links">
              <div>
                <strong>星渡</strong>
                <a href="/#features">产品能力</a>
                <a href="/pricing">价格与部署</a>
                <a href="/app">控制台</a>
              </div>
              <div>
                <strong>了解更多</strong>
                <a href="/privacy">隐私说明</a>
                <a href="/security">安全设计</a>
                <a href={docs}>部署文档</a>
              </div>
              <div>
                <strong>开放生态</strong>
                <a href={repository}>
                  GitHub <Arrow />
                </a>
                <a href={`${repository}/releases`}>
                  版本动态 <Arrow />
                </a>
                <a href={`${repository}/blob/main/LICENSE`}>
                  MIT 许可证 <Arrow />
                </a>
              </div>
            </div>
          </div>
          <div className="site-footer-bottom">
            <span>© 2026 星渡 Xingdu</span>
            <span>开源可用 · 托管服务筹备中</span>
            <span>Built to connect.</span>
          </div>
        </div>
      </footer>
    </div>
  );
}
