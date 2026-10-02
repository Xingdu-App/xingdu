# Agent 版本与发布

`VERSION` 是版本化构建的唯一版本来源。`make build`、`make agents` 和控制端
Docker 镜像将同一版本注入 API、Worker 与 Agent；直接 `go run` 保留开发版
标签。GitHub 发布包不再固定显示 `-dev`。候选版使用 `0.14.0-rc.1` 形式，
稳定版使用 `0.14.0`，发布 tag 对应 `agent-v0.14.0-rc.1` 或 `agent-v0.14.0`。

## 源码与发布包的边界

同一个 VERSION 下仍可能有 tag 之后的新提交。判断发布包包含哪些功能，
须核对 tag 指向的源码提交，不能只看版本文本。`agent-v0.14.0-rc.1` 不包含后续的 SELinux 修复和 TrustTunnel 支持。
此前候选版本 `agent-v0.15.0-rc.1` 包含这些改动；发布完成状态以 GitHub Release
和对应工作流为准，新功能验收与旧包可用性分开记录。
不要覆盖已公开的包或重复使用同一个 tag，见 [功能状态](IMPLEMENTATION-STATUS.md)。

## 发布步骤

1. 更新 VERSION，将改动提交并合入 main；本地执行 `make check`。
2. 推送对应版本 tag。工作流拒绝非 tag、版本不匹配、不在 main 历史中的提交。
3. Agent Release 工作流先复用完整 CI：Go 检查、PostgreSQL 集成测试、前端
   lint/测试/构建。通过后在原生 Linux amd64 和 arm64 runner 执行 Agent
   相关测试，构建静态二进制，并实际执行 `--version` 验证版本。
4. 使用 GitHub OIDC 生成构建来源证明，汇总 SHA-256、版本、源码提交和 MIT
   许可证。所有文件上传到 draft 后才公开 Release；候选版标记 prerelease，
   不替换稳定版 latest。重复运行只可继续未公开 draft，不覆盖已公开 Release。
5. 同一 tag 的 API/Worker 镜像部署完成后，再核对公开下载端点、可用版本及
   目标机器新心跳。管理员主动触发升级；发布不会自动升级现有机器。

手动重跑时，在 Actions 的 Agent Release 中选择既有 tag。无需额外个人 token，
使用工作流自己的 GITHUB_TOKEN。仓库规则仍需限制 main、发布 tag 和发布权限。

```sh
git tag -a agent-v0.14.0-rc.1 -m 'Release Xingdu Agent 0.14.0-rc.1'
git push origin agent-v0.14.0-rc.1
```

## 下载验证

下载与架构对应的二进制和 SHA256SUMS，在同一目录执行：

```sh
sha256sum --ignore-missing -c SHA256SUMS
gh attestation verify xingdu-agent-linux-amd64 --repo Xingdu-App/xingdu
chmod 0755 xingdu-agent-linux-amd64
./xingdu-agent-linux-amd64 --version
```

SHA-256 用于发现下载损坏；GitHub 来源证明绑定构建仓库、工作流和源码，
不代表功能或生产适用性验收。当前机器自更新仍通过受信控制端 HTTPS 与固定
SHA-256 校验，尚不自动验证 GitHub attestation。不要把来源证明描述为已落地
的 Agent 更新签名校验。

## 首版边界

首版以 release candidate 发布。原生 runner 的单元测试及版本命令不等于
公网 VPS 上的 systemd 安装、协议转发、断网恢复或真实客户端验收。这些验收
结果单独记录于 MACHINE-ACCESS.md、AGENT-LAB.md 和协议文档。

## 0.16.0-rc.1 官方运行时切换

新 Agent 安装未经修改的官方 sing-box 1.14.2 和 Xray 26.9.9，使用独立的
`/api/v1/agent/runtime/official/{family}/{arch}` 下载入口，并核对归档和
可执行文件摘要。旧下载入口保留历史二进制，避免旧 Agent 固定摘要失效。
XHTTP 仍使用星渡管理的前端和官方 Xray 后端，不等于原生 Xray 端到端拓扑。

新建服务以 systemd cgroup BPF 过滤私有、元数据等地址；过滤未生效时拒绝
启动并清理服务。该限制同时作用于入站，私有来源的客户端连接也会被拒绝。
需要支持此过滤机制的 Linux/systemd 环境。DNS 使用公共解析器。

已有节点不会自动迁移：升级 Agent 本身或普通配置更新不会重写已有服务单元。
需要在备份配置、确认维护窗口后重新部署节点，才能切换运行时和服务策略。
发布候选版不等于已有 VPS 或客户端组合已经验收。

本地验证覆盖 72 项配置解析、17 项 sing-box 转发，以及真实 Ubuntu arm64
systemd BPF 查询和同一 UDP socket 的公有/私有目标切换。72 项配置通过
不等于 72 项端到端转发通过。原生架构构建与 CI 以发布工作流结果为准。
