## English

- Use an external SOCKS5 or HTTP CONNECT proxy as a node’s outbound exit.
- Manage external proxy credentials securely in the console, without exposing
  them in client subscriptions.
- Keep active exits protected from editing or deletion, and prevent older
  Agents from accepting tasks that require external proxy support.

External exits currently support TCP and public IPv4 or IPv6 addresses.
UDP forwarding and hostname endpoints are not supported. Existing machines
require a manual Agent upgrade; update the API and Worker together to use
these controls. Existing nodes keep their installed engines and settings.
Local forwarding tests do not establish acceptance of a specific ISP proxy,
public VPS or client App.

## 简体中文

- 支持将外部 SOCKS5 或 HTTP CONNECT 代理设为节点的出口。
- 在控制台安全管理外部代理凭据，客户端订阅不会包含这些凭据。
- 使用中的出口禁止编辑或删除；旧版 Agent 不会领取需要外部出口的任务。

外部出口目前支持 TCP 和公网 IPv4／IPv6 地址，暂不支持 UDP 转发或域名地址。
现有机器需手动升级 Agent，API 与 Worker 需同步更新才能使用新控制项。
已有节点保留原有内核和服务配置。本地转发测试不代表具体 ISP 代理、公网 VPS
或客户端 App 已通过验收。

## 繁體中文

- 支援將外部 SOCKS5 或 HTTP CONNECT 代理設為節點的出口。
- 在控制台安全管理外部代理憑據，客戶端訂閱不會包含這些憑據。
- 使用中的出口禁止編輯或刪除；舊版 Agent 不會領取需要外部出口的任務。

外部出口目前支援 TCP 和公網 IPv4／IPv6 位址，暫不支援 UDP 轉發或網域位址。
現有機器需手動升級 Agent，API 與 Worker 需同步更新才能使用新控制項。
現有節點保留原有核心和服務設定。本地轉發測試不代表個別 ISP 代理、公網 VPS
或客戶端 App 已通過驗收。

Linux downloads are provided for amd64 and arm64. Verify SHA256SUMS before
installation. Build provenance can be checked with
`gh attestation verify BINARY --repo Xingdu-App/xingdu`.
See [installation and release guidance](https://github.com/Xingdu-App/xingdu/blob/agent-v0.18.0/docs/AGENT-RELEASE.md).
