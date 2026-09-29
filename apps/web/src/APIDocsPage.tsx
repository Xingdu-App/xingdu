import { useLocale } from "./i18n";
import "./api-docs.css";

export default function APIDocsPage({ onKeys }: { onKeys: () => void }) {
  const locale = useLocale();
  const copy = (zh: string, en: string) => (locale === "en" ? en : zh);
  const base = window.location.origin;
  const endpoints = [
    ["GET", "/hosts", "hosts:read", copy("列出服务器", "List servers")],
    [
      "POST",
      "/hosts",
      "hosts:write",
      copy("创建服务器记录", "Create server inventory"),
    ],
    [
      "PUT",
      "/hosts/{srv_id}",
      "hosts:write",
      copy("更新服务器资料", "Update server metadata"),
    ],
    [
      "DELETE",
      "/hosts/{srv_id}",
      "hosts:write",
      copy("删除服务器记录", "Delete server inventory"),
    ],
    [
      "GET",
      "/nodes",
      "nodes:read",
      copy("列出组织节点", "List organization nodes"),
    ],
    [
      "GET",
      "/hosts/{srv_id}/deployments",
      "nodes:read",
      copy("查询部署及任务状态", "Read deployments and job state"),
    ],
    [
      "POST",
      "/hosts/{srv_id}/deployments/preflight",
      "nodes:write",
      copy("安装预检", "Installation preflight"),
    ],
    [
      "POST",
      "/hosts/{srv_id}/deployments",
      "nodes:write",
      copy("提交安装任务", "Queue installation"),
    ],
    [
      "PUT",
      "/hosts/{srv_id}/deployments/{node_id}",
      "nodes:write",
      copy(
        "更新配置；需要 confirm: true",
        "Update configuration; requires confirm: true",
      ),
    ],
    [
      "POST",
      "/hosts/{srv_id}/deployments/{node_id}/restart",
      "nodes:write",
      copy(
        '重启；请求体为 {"confirm":true}',
        'Restart; body: {"confirm":true}',
      ),
    ],
    [
      "DELETE",
      "/hosts/{srv_id}/deployments/{node_id}",
      "nodes:write",
      copy("提交卸载任务", "Queue uninstall"),
    ],
    [
      "POST",
      "/hosts/{srv_id}/deployments/{node_id}/connection",
      "nodes:credentials",
      copy(
        "读取连接凭据；请求体为 {}",
        "Read connection credentials; body: {}",
      ),
    ],
    [
      "POST",
      "/hosts/{srv_id}/deployments/{node_id}/probe",
      "nodes:probe",
      copy(
        "上报客户端探测结果，不会发起探测",
        "Report a client probe result; does not run a probe",
      ),
    ],
  ];
  const example = `import getpass\nimport json\nimport urllib.request\n\nbase = ${JSON.stringify(base)}\nkey = getpass.getpass("Xingdu API key: ")\n\ndef request(method, path, payload=None):\n    body = None if method == "GET" else json.dumps(payload or {}).encode()\n    req = urllib.request.Request(\n        base + "/api/v1" + path, data=body, method=method,\n        headers={\n            "Authorization": "Bearer " + key,\n            "Content-Type": "application/json",\n        },\n    )\n    with urllib.request.urlopen(req, timeout=30) as response:\n        return None if response.status == 204 else json.load(response)\n\nfor host in request("GET", "/hosts")["data"]:\n    print(host["id"], host["name"])\nfor node in request("GET", "/nodes")["data"]:\n    print(node["id"], node["name"])`;
  return (
    <article className="api-docs">
      <section className="panel">
        <h2>{copy("开始使用", "Getting started")}</h2>
        <p>
          {copy(
            "使用 API 自动管理当前组织的服务器和节点。由组织所有者或管理员创建密钥，按需授予权限。",
            "Automate server and node management in your organization. An owner or administrator creates a key with the required permissions.",
          )}
        </p>
        <p>
          {copy("接口地址", "Base URL")}：<code>{base}/api/v1</code>
        </p>
        <button className="secondary" onClick={onKeys}>
          {copy("管理 API 密钥 →", "Manage API keys →")}
        </button>
        <nav aria-label={copy("API 文档目录", "API documentation sections")}>
          <a href="#api-auth">{copy("认证", "Authentication")}</a>
          <a href="#api-example">{copy("快速开始", "Quick start")}</a>
          <a href="#api-endpoints">
            {copy("接口与权限", "Endpoints and scopes")}
          </a>
          <a href="#api-deployment">{copy("部署节点", "Deploy a node")}</a>
          <a href="#api-errors">{copy("响应与错误", "Responses and errors")}</a>
        </nav>
      </section>
      <section className="panel" id="api-auth">
        <h2>{copy("认证与权限", "Authentication and permissions")}</h2>
        <pre>
          <code>
            {"Authorization: Bearer <API_KEY>\nContent-Type: application/json"}
          </code>
        </pre>
        <p>
          {copy(
            "组织由密钥自动确定，无需组织参数。不要混用 Cookie、Origin 或浏览器 Fetch 请求头；API 密钥用于服务端脚本，不用于网页前端。生产环境请使用 HTTPS。",
            "The key selects the organization; no organization parameter is needed. Do not send Cookie, Origin or browser Fetch metadata headers. Use keys in server-side scripts, not browser applications. Use HTTPS in production.",
          )}
        </p>
        <p>
          {copy(
            "各权限独立：写入权限不包含读取权限，也不包含连接凭据读取权限。密钥不能用于管理账号、账单、订阅、API 密钥、SSH 凭据或 Agent 身份。",
            "Scopes are independent: write access does not imply read or credential access. Keys cannot manage accounts, billing, subscriptions, API keys, SSH credentials or Agent identities.",
          )}
        </p>
        <p>
          {copy(
            "完整密钥仅在创建时显示，有效期为 1–365 天。创建者退出组织或失去管理员权限后不可使用；永久停用请撤销密钥。撤销不会取消已排队的远程任务。",
            "The full key is shown only at creation and expires in 1–365 days. It stops working if its creator leaves the organization or loses administrator access. Revoke it for permanent invalidation; revocation does not cancel queued remote jobs.",
          )}
        </p>
      </section>
      <section className="panel" id="api-example">
        <h2>{copy("快速开始 · Python", "Quick start · Python")}</h2>
        <p>
          {copy(
            "仅使用 Python 标准库，需要 hosts:read 和 nodes:read。运行后安全输入密钥；无人值守脚本请从密钥管理服务读取，不要写入源码或日志。",
            "Uses only the Python standard library. Requires hosts:read and nodes:read. Enter the key at the hidden prompt; unattended scripts should use a secret manager, never source code or logs.",
          )}
        </p>
        <pre>
          <code>{example}</code>
        </pre>
      </section>
      <section className="panel" id="api-endpoints">
        <h2>{copy("接口与权限", "Endpoints and scopes")}</h2>
        <p>
          {copy(
            "下列路径均以 /api/v1 开头。srv_id 和 node_id 使用列表接口返回的真实资源 ID。",
            "All paths below start with /api/v1. Use resource IDs returned by list endpoints for srv_id and node_id.",
          )}
        </p>
        <div
          className="api-docs-table"
          tabIndex={0}
          role="region"
          aria-label={copy("接口列表", "Endpoint list")}
        >
          <table>
            <thead>
              <tr>
                {[
                  copy("方法", "Method"),
                  copy("路径", "Path"),
                  copy("权限", "Scope"),
                  copy("用途", "Purpose"),
                ].map((h) => (
                  <th key={h} scope="col">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {endpoints.map(([method, path, scope, description]) => (
                <tr key={method + path}>
                  <td>
                    <code>{method}</code>
                  </td>
                  <td>
                    <code>{path}</code>
                  </td>
                  <td>
                    <code>{scope}</code>
                  </td>
                  <td>{description}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
      <section className="panel">
        <h2>{copy("常用请求体", "Common request bodies")}</h2>
        <p>
          {copy(
            "创建或更新服务器资料（POST /hosts 或 PUT /hosts/{srv_id}）：",
            "Create or update server metadata (POST /hosts or PUT /hosts/{srv_id}):",
          )}
        </p>
        <pre>
          <code>
            {JSON.stringify(
              {
                name: "Automation server",
                address: "vps.example.com",
                ssh_port: 22,
                ssh_user: "root",
                tags: ["automation"],
                notes: "",
              },
              null,
              2,
            )}
          </code>
        </pre>
        <p>
          {copy(
            "探测上报字段：ok（是否成功）、latency_ms（0–120000）、exit_ip（成功时的实际公网出口）、revision（当前节点版本）。这些值由你的客户端实测产生，版本不匹配会返回 409。",
            "Probe report fields: ok (success), latency_ms (0–120000), exit_ip (observed public exit on success), revision (current node revision). Supply actual client observations; a revision mismatch returns 409.",
          )}
        </p>
      </section>
      <section className="panel" id="api-deployment">
        <h2>{copy("部署节点", "Deploy a node")}</h2>
        <p>
          {copy(
            "先在控制台接入托管 Agent。创建服务器记录本身不会安装 Agent 或建立 SSH 连接。以下为 Trojan 安装请求体，证书与私钥须替换为完整 PEM 内容。",
            "Enroll a managed Agent in the console first. Creating inventory does not install an Agent or establish SSH access. This Trojan installation body requires full PEM certificate and private-key contents.",
          )}
        </p>
        <pre>
          <code>
            {JSON.stringify(
              {
                name: "Automation node",
                protocol: "trojan",
                port: 443,
                server_name: "node.example.com",
                certificate: "<PEM certificate chain>",
                private_key: "<PEM private key>",
                confirm_install: true,
              },
              null,
              2,
            )}
          </code>
        </pre>
        <ol>
          <li>
            {copy(
              "向 /hosts/{srv_id}/deployments/preflight 提交上述字段，去掉 confirm_install。预检检查控制端条件和已登记端口，不验证公网连接。",
              "POST these fields without confirm_install to /hosts/{srv_id}/deployments/preflight. Preflight checks control-plane requirements and inventory ports, not public connectivity.",
            )}
          </li>
          <li>
            {copy(
              "向 /hosts/{srv_id}/deployments 提交完整请求体。HTTP 202 表示任务已排队。",
              "POST the full body to /hosts/{srv_id}/deployments. HTTP 202 means the job was queued.",
            )}
          </li>
          <li>
            {copy(
              "定期 GET /hosts/{srv_id}/deployments，查看返回的 state。请求超时后先查询状态，避免重复安装。任务完成后再用客户端验证线路。",
              "Poll GET /hosts/{srv_id}/deployments and inspect state. After a timeout, check state before retrying installation. Validate the route with a client after completion.",
            )}
          </li>
        </ol>
        <p>
          {copy(
            "协议值：shadowsocks、shadowsocks2022、trojan、vless、vmess、hysteria2、tuic、anytls、http。Shadowsocks 两种模式无需 TLS 字段；http 为 HTTPS 代理，仍需要证书。可用性取决于 Agent 版本与服务端运行时。",
            "Protocol values: shadowsocks, shadowsocks2022, trojan, vless, vmess, hysteria2, tuic, anytls, http. Both Shadowsocks modes omit TLS fields; http is an HTTPS proxy and requires a certificate. Availability depends on the Agent version and server runtime.",
          )}
        </p>
        <p>
          {copy(
            '更新示例：PUT /hosts/{srv_id}/deployments/{node_id}，请求体 {"port":8443,"confirm":true}。更新可能短暂中断连接。',
            'Update example: PUT /hosts/{srv_id}/deployments/{node_id} with {"port":8443,"confirm":true}. Updates may briefly interrupt connections.',
          )}
        </p>
      </section>
      <section className="panel" id="api-errors">
        <h2>{copy("响应、错误与限流", "Responses, errors and rate limits")}</h2>
        <p>
          {copy(
            '资源响应通常为 {"data":...}；HTTP 204 没有响应体。探测上报成功返回 {"ok":true}。所有写请求（含 DELETE）均须声明 Content-Type: application/json。',
            'Resource responses usually use {"data":...}; HTTP 204 has no body. Successful probe reports return {"ok":true}. All mutations, including DELETE, require Content-Type: application/json.',
          )}
        </p>
        <pre>
          <code>
            {'{"error":{"code":"insufficient_scope","message":"..."}}'}
          </code>
        </pre>
        <ul>
          <li>
            {copy(
              "401：密钥无效、过期、已撤销或创建者不再具有权限。",
              "401: invalid, expired or revoked key, or creator no longer authorized.",
            )}
          </li>
          <li>
            {copy(
              "403：缺少授权范围、接口不允许 API 密钥访问或混用了浏览器身份。",
              "403: missing scope, endpoint not available to API keys, or mixed browser identity.",
            )}
          </li>
          <li>
            {copy(
              "404 / 409 / 422：资源不存在、当前状态冲突或参数无效；按 error.code 和 error.message 修正请求。",
              "404 / 409 / 422: resource missing, state conflict or invalid input; inspect error.code and error.message.",
            )}
          </li>
          <li>
            {copy(
              "429：触发限流。API 密钥上限为每个 API 进程每分钟 120 次，部分操作另有限制。优先遵循 Retry-After，未提供时退避重试。",
              "429: rate limited. Each key allows 120 requests per minute per API process; some operations have additional limits. Honor Retry-After when present, otherwise back off.",
            )}
          </li>
        </ul>
      </section>
    </article>
  );
}
