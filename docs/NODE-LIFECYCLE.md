# 节点检查、维护与受控重启

节点生命周期包括预检、服务状态、重启、配置更新与历史恢复。基础协议部署
最低 Agent 版本见 [协议矩阵](PROTOCOL-DEPLOYMENT.md)，配置更新/恢复至少
需要 0.12.0-dev；新增协议还须满足各自的版本门槛。管理员可主动升级 Agent，
但发布不会自动升级机器。升级保留机器身份与已有服务，不能删除仍在使用的
`/var/lib/xingdu-agent`，详见 [机器升级](MACHINE-ACCESS.md#网页升级-agent)。

## 配置预检与证书

新建节点的「检查配置」验证协议、端口范围、证书与私钥匹配、TLS 域名覆盖、证书当前有效、组织管理权限、托管 Agent 就绪、该机器是否存在未完成操作和已占用的登记端口。预检不会保存证书或私钥。创建任务时再次进行检查，避免把预检结果当作锁定资源的承诺。

端口登记检查不等于操作系统监听检查；真实占用在 Agent 执行安装时检查。DNS、公网路由、云安全组和防火墙仍需验证。已提供运营者独立运行的 [ACME 签发/续期程序](AUTOMATIC-CERTIFICATES.md)，尚无租户自助 DNS 授权；不会自动修改防火墙。

新部署保存公开证书的到期时间，在节点卡片和详情展示；30 天内显示到期提醒，过期显示“已过期”。旧记录没有自动解密回填，显示“待采集”。这只是控制台提醒，不会自动发送外部通知。

## 本机服务检查

Agent 大约每 30 秒从认证接口获取本机已安装节点 ID，执行受限的 systemd 服务状态查询，并读取受管运行时版本及检查 SELinux 进程域。检查前验证星渡服务与状态目录的归属，不接受任意命令或路径。报告包含节点 ID、有界服务状态和可确认的运行时版本，不上传日志、配置或凭据；启用 SELinux 的服务未进入专用域时报告 `policy_required`，即「安全策略待修复」。后台检查只读，不执行修复。控制端使用服务端接收时间，超过 90 秒未收到检查结果显示“等待检查”，不继续声称当前正在运行。

“运行中”仅说明 systemd 服务处于 active 状态；不证明监听端口、TLS 握手、认证转发、公网可达性或客户端兼容性。机器 Agent 心跳、本机服务状态和外部协议连通性是不同事实。

## 重启

组织所有者/管理员可确认短暂中断连接后提交重启。后台要求在线且满足节点协议版本门槛的托管 Agent、已确认安装的节点、无其他正在执行的操作。Agent 重启对应的星渡服务，保留协议配置与凭据。包含 SELinux 修复的新 Agent 会在部署、更新和主动重启前准备共享策略、校验程序与当前配置标签；工具缺失时可通过现有 dnf/yum 源安装所需包。流程不关闭 SELinux，启动后验证实际进程域；详见 [SELinux 检测与修复](PROTOCOL-DEPLOYMENT.md#selinux-检测与修复)。旧发布包不具备该修复能力。

重启继承任务租约、幂等操作日志、撤销授权检查和单机器排队。报告丢失重试不会再次执行重启；执行结果不确定时标记中断，不自动重复。失败/中断不会删除节点或加密配置。现有客户端凭据保持不变。

配置编辑、证书替换、凭据轮换、配置修订、更新启动失败恢复及历史恢复已实现，详见 [可靠部署](RELIABLE-DEPLOYMENTS.md)。历史恢复创建新修订；恢复旧凭据会重新允许其访问。协议引擎升级与这些配置操作是独立能力，不由 Agent 升级或重启自动完成。

## API

- `POST /api/v1/hosts/{id}/deployments/preflight`：管理员配置预检，输入与新建协议一致，但无 `confirm_install`。
- `POST /api/v1/hosts/{id}/deployments/{deployment}/restart`：管理员重启，需 `{ "confirm": true }`。
- `GET /api/v1/agent/deployments/status`：机器身份读取本机节点 ID。
- `POST /api/v1/agent/deployments/status`：机器身份提交最多 100 条固定状态；跨机器、未知节点及重复 ID 拒绝。

列表新增 `certificate_expires_at`、`service_status`、`service_checked_at`，及 `runtime_version`，均不含机密。

## 验证

`python3 scripts/node-lifecycle-lab.py` 针对已有 Ubuntu 实验机器和演示节点执行受控验收，保留注册身份和节点，不卸载现有服务。2026-09-28 arm64 Docker 验收通过：升级至 Agent 0.6.0-dev、两个既有节点报告运行中、停止 Hysteria 2 后报告未运行、通过 API 重启恢复、真实 QUIC 认证转发和私有目的地址阻止通过，节点 ID/安装时间不变。报告保存在本地 `.local/agent-lab/node-lifecycle-report.json`。此项不代表真实 VPS、公网网络、Debian/Amazon 新生命周期逻辑或实际客户端 App 已验收。

2026-09-30 的 AlmaLinux Enforcing / SS2022 实机修复及 Stash 节点测试记录见 [协议部署](PROTOCOL-DEPLOYMENT.md#selinux-检测与修复)。该结果独立于上述历史容器测试，不覆盖所有生命周期或客户端组合。
