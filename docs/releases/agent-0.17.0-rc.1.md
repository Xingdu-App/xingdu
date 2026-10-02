## English

- Add VLESS REALITY with optional Vision, using generated keys without uploading a certificate.
- Add Trojan transport choices and deployment presets, with transport editing for existing nodes.
- Add sing-box JSON, URI and Base64 subscriptions. Unsupported combinations return clear errors.
- Prevent older Agents from accepting tasks that require newer features.

This is a release candidate. Existing machines require a manual Agent upgrade.
Upgrade the API and Worker together to expose the new controls and subscriptions.
Existing nodes keep their installed engines and service settings; some older
nodes require redeployment before using new features.

Use a compatible client: the current sing-box client cannot connect to the
new Xray REALITY server. Use a compatible Xray client for that combination.
Public VPS and individual client App acceptance remain separate from local tests.

## 简体中文

- 新增 VLESS REALITY，可选 Vision，自动生成密钥，无需上传证书。
- 新增 Trojan 传输选项与部署预设，支持编辑已有节点的传输配置。
- 新增 sing-box JSON、URI 和 Base64 订阅；不兼容组合会明确报错。
- 阻止旧版 Agent 领取需要新版功能的任务。

此版本为候选版。现有机器需手动升级 Agent，API 与 Worker 也需同步更新，
才能提供新控制项与订阅。已有节点保留原有内核和服务配置；部分旧节点
使用新功能前需重新部署。

请使用兼容客户端：当前 sing-box 客户端无法连接新版 Xray REALITY 服务端，
该组合需使用兼容的 Xray 客户端。公网 VPS 与具体客户端 App 仍需独立验收。

## 繁體中文

- 新增 VLESS REALITY，可選 Vision，自動產生金鑰，無需上傳憑證。
- 新增 Trojan 傳輸選項與部署預設，支援編輯現有節點的傳輸設定。
- 新增 sing-box JSON、URI 和 Base64 訂閱；不相容組合會明確報錯。
- 防止舊版 Agent 領取需要新版功能的任務。

此版本為候選版。現有機器需手動升級 Agent，API 與 Worker 也需同步更新，
才能提供新控制項與訂閱。現有節點保留原有核心和服務設定；部分舊節點
使用新功能前需重新部署。

請使用相容客戶端：目前 sing-box 客戶端無法連接新版 Xray REALITY 伺服器，
該組合需使用相容的 Xray 客戶端。公網 VPS 與個別客戶端 App 仍需獨立驗收。

Linux downloads are provided for amd64 and arm64. Verify SHA256SUMS before
installation. Build provenance can be checked with
`gh attestation verify BINARY --repo Xingdu-App/xingdu`.
See [installation and release guidance](https://github.com/Xingdu-App/xingdu/blob/agent-v0.17.0-rc.1/docs/AGENT-RELEASE.md).
