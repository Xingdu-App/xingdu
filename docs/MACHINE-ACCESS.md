# 机器接入与凭据安全

星渡提供两种接入路径，共用一套机器身份、心跳和撤销机制。当前实现机器安装、探针及托管 Agent 的固定协议部署任务，不包含任意远程 Shell 或完整运维任务执行。协议安装见 [协议部署](PROTOCOL-DEPLOYMENT.md)。

| 接入方式 | 用户提供 | 控制端执行 |
| --- | --- | --- |
| 主动安装 | 在 VPS 执行安装脚本并输入短期令牌 | 接收注册和主动上报的心跳，无须 SSH 凭据 |
| SSH 密码 | 服务器地址、账号、密码、已核实主机指纹 | 校验指纹后认证，上传固定安装包，安装 systemd 服务 |
| SSH 私钥 | PEM / OpenSSH 私钥，可选私钥口令、已核实指纹 | 同上；不要求解密私钥后再上传 |
| 手动二进制 | 自行构建或下载校验过的 Agent | `--init` 后前台运行，适用于调试及自定义服务管理器 |

Linux systemd 安装支持 amd64 / arm64，首批目标为 Debian / Ubuntu 系。需要 curl、sha256sum、systemd、useradd；SSH 安装缺少 tar 时会通过现有 apt-get、dnf、yum 或 zypper 自动安装 tar，非 root SSH 用户需要免密 sudo。不会修改 SSH 服务、安全组或防火墙，不接收 sudo 密码，不覆盖已有 Agent。

## 权限模式

- **探针 monitor**：systemd 使用专用 `xingdu-agent` 系统用户，清空 capabilities，启用 NoNewPrivileges、只读系统目录和私有临时目录；仅能写自己的状态目录。只采集主机名、系统与架构、Agent 版本、运行时间、CPU 核数、内存和负载，不上传环境变量、进程列表、文件内容或 SSH 凭据。
- **托管 manage（默认）**：用户明确确认后以 root 运行服务，执行明确授权的协议安装与卸载任务，并上报状态。**托管模式不等于任意远程控制**。控制端暂不提供任意命令输入框。

组织所有者、管理员可以安装、签发令牌、使用/删除凭据及撤销机器。普通成员可以新增资料并查看探针，但不能修改 SSH 连接目标、删除机器、安装或读取凭据；只读成员仅可查看。敏感操作写入组织隔离的 `machine_audit`，组织所有者与管理员可在设置页面查看最近的机器与订阅审计记录。

控制台主动安装与 SSH 安装、安装脚本及命令行初始化均默认使用托管模式。已有 Agent 继续使用其保存的模式，不会自动切换；探针模式可显式指定 `--mode monitor`。

## 主动安装

1. 新增服务器资料，打开「接入 / 状态」。
2. 默认选择托管模式，确认 root 运行授权后生成一次性令牌，复制下载命令；只需采集状态时可改选探针模式。
3. 在目标 VPS 查看脚本内容，执行脚本，在隐藏输入提示中粘贴令牌。
4. 以控制台的实际心跳判断在线，不能仅以安装命令完成判断接入成功。

控制端构建并提供两个架构的二进制，安装脚本内嵌本次构建的 SHA-256，下载后校验，不执行未校验的二进制。校验和与脚本来自同一 HTTPS 来源，不能替代独立发布签名；发布 CI 已生成 GitHub OIDC 来源证明，但安装/更新器尚不自动验证该证明，见 [发布说明](AGENT-RELEASE.md)。不要使用不可信镜像或跳过 TLS 校验。

安装地址：

- `GET /api/v1/agent/install.sh`
- `GET /api/v1/agent/download/amd64`
- `GET /api/v1/agent/download/arm64`

构建本地二进制：`make agents`。Go 热更新服务需配置 `XINGDU_AGENT_ARTIFACT_DIR` 为构建产物目录；Docker 镜像已包含构建产物。下载端点公开但不携带注册令牌。

自定义服务管理器示例（先用可信方式取得二进制）：

```sh
./xingdu-agent --init --config ./agent.json --server https://control.example.com --mode manage
./xingdu-agent --config ./agent.json
```

`--init` 从终端隐藏读取令牌；自动化可经 stdin 传入，不支持令牌命令行参数。配置文件必须是普通文件、0600 权限，Agent 拒绝读取可被其他用户读取的配置。systemd 状态目录为 `/var/lib/xingdu-agent`（0700），配置文件由服务用户持有。

注册失败会保留初始身份，便于网络恢复后用同一配置重试；若令牌已经过期或撤销，先检查并清理这次失败安装的状态目录，再申请新令牌。不要直接覆盖未知的已有安装。服务启动失败时检查 `journalctl -u xingdu-agent`。

## 网页升级 Agent

在服务器「接入 / 状态」中查看当前版本与控制端可用版本，点击「升级 Agent」。从 0.11.0-dev 起，标准 systemd 安装包含独立升级服务；服务正常连接时优先使用 **Agent 自更新，不需要 SSH 密码、PEM 或保存凭据**。任务必须由组织所有者、管理员主动提交，不会因为发布新版本就自动升级。

旧 Agent 需要先通过 SSH 升级一次，或由管理员在机器上更新到支持自更新的版本。已有加密保存的 SSH 凭据可直接使用；没有时输入一次密码或 PEM 并核实指纹。升级服务未连接或故障时仍保留 SSH 恢复入口。仅比较版本大小不足以判断自更新可用性，网页以升级服务最近 90 秒的认证连接为准。

标准安装创建 `xingdu-agent-update.service` 和 `xingdu-agent-update.timer`，约每 30 秒检查是否有授权任务。这个独立的 root 服务负责校验、替换及重启 Agent，不随 Agent 重启被终止。探针主服务仍使用低权限用户和原有沙箱；升级服务的信任配置保存在 root 所有、0600 的 `/etc/xingdu-agent/updater.json`，普通探针用户不能改写控制端来源或升级程序。此配置复用机器身份，不包含 SSH 凭据。只使用标准固定下载端点，不接受租户指定 URL 或命令。

升级任务绑定组织、机器身份、架构、目标版本和升级包 SHA-256。机器主动领取任务，下载后、执行前再次校验发起者管理权限和任务租约；拒绝摘要错误、重定向、架构不匹配和降级。旧 SSH 路径同样校验身份和升级包。两条路径共用安装任务唯一约束，升级期间阻止新协议操作；已有协议任务未结束时不能排队升级。

升级保留 `/var/lib/xingdu-agent/agent.json`、机器身份、权限模式和已有协议服务。需要标准 Xingdu systemd 安装；自定义 unit、drop-in、控制端地址或身份不匹配会被拒绝。新二进制原子替换旧文件，只重启 Agent；启动失败会尝试恢复旧二进制并重启，恢复失败需人工处理。自更新需先确认升级程序执行成功，再收到同一机器身份的目标版本新心跳才完成。任务 5 分钟内未完成会显示失败/未确认，不自动重复执行。强制终止、掉电或网络中断可能留下需要人工检查的不确定结果。

`POST /api/v1/hosts/{id}/agent/upgrade` 创建无凭据自更新任务；`POST /api/v1/hosts/{id}/ssh/upgrade` 为兼容和恢复入口。两者必须提供 `confirm_upgrade: true`。自更新机器接口拒绝浏览器 Cookie/Origin，使用独立机器 bearer；领取、执行前检查和完成均在事务局部租户上下文及 RLS 下执行，SSH Worker 不会领取自更新任务。撤销身份或发起者权限会阻止后续执行，但无法撤回已开始的远程操作。发布包已有 GitHub OIDC 来源证明，但自更新尚不自动验证该证明；当前信任控制端 HTTPS 来源与其固定 SHA-256。

2026-09-29 的隔离 arm64 Docker 验证覆盖 Ubuntu 24.04、Debian 13 和 Amazon Linux 2023 从 0.7.0-dev 升级到控制端构建版本：一次性与保存凭据、身份及模式保留、目标心跳确认均通过；两个托管系统验证已部署节点在升级后继续转发，探针系统保持低权限运行。另以故意启动失败的测试二进制验证真实 systemd 回滚。该结果不代表公网 VPS 或 amd64 实机验收。

补充自更新验收：三个隔离 arm64 系统均在停止 SSH 服务后，通过网页 API 创建无凭据任务并升级成功。测试使用带自更新能力的旧版本标签构建作为夹具，不能据此认为已发布的旧 Agent 具备该能力。托管节点继续转发，Amazon Linux 探针保持低权限运行，探针用户无法读取 root 升级配置；摘要错误、重定向、重复领取、跨租户/错误租约和管理员撤权另有自动化覆盖。未验证公网 VPS 或真实客户端 App。

## 身份、心跳与撤销

手动注册令牌绑定组织、机器和权限模式，15 分钟有效且只能使用一次；SSH 任务的注册令牌有效期 30 分钟。数据库仅保存令牌摘要。Agent 在本地生成并先保存独立随机机器凭据，再注册；即使注册响应丢失，也可凭该身份恢复。注册完成后从配置中清除初始令牌。

Agent 使用 HTTPS 和独立 bearer 凭据，每 30 秒主动发送心跳，无须开放 VPS 入站端口。当前不是 mTLS。TLS 校验不可跳过，不跟随控制端重定向；HTTP 仅允许回环地址用于本机测试。机器接口拒绝浏览器 Cookie/Origin，机器凭据不能访问用户管理接口，心跳内容不能选择其他机器或组织。

超过 90 秒没有心跳显示离线；在线时间来自服务端时钟。控制台每 15 秒刷新列表。撤销会使后续心跳失败，Agent 正常退出并停止自动重启。注册新身份会替换旧身份；更换模式需要重新注册。

需要停止协议服务时，应先在「协议部署」中卸载并确认完成，再撤销机器接入。撤销不会停止已经运行的协议服务。

「撤销机器接入」同时撤销未使用令牌、旧 Agent 身份并取消安装任务，**不会卸载 VPS 上的服务，也不能撤回已发出的远程操作**。删除资料会级联撤销相关身份、任务和保存的凭据。卸载或重装 Agent 前，请先卸载该机器的协议服务并确认完成；不要删除仍被协议服务使用的 `/var/lib/xingdu-agent/protocols`。需要卸载 Agent 时，由管理员在 VPS 上执行：

```sh
sudo systemctl disable --now xingdu-agent-update.timer xingdu-agent-update.service
sudo systemctl disable --now xingdu-agent
sudo rm /etc/systemd/system/xingdu-agent-update.service /etc/systemd/system/xingdu-agent-update.timer
sudo rm /etc/xingdu-agent/updater.json
sudo rmdir /etc/xingdu-agent
sudo rm /etc/systemd/system/xingdu-agent.service
sudo systemctl daemon-reload
sudo rm /usr/local/bin/xingdu-agent
# 核实目录属于已撤销的星渡安装后，再清理 /var/lib/xingdu-agent。
```

## SSH 信任与网络边界

指纹检查仅读取主机公钥，不发送密码或私钥。用户必须通过云控制台或已有可信连接核对 SHA256 指纹，然后明确确认。自动扫描获得的指纹本身不证明机器身份。安装重新连接时严格匹配公钥；变更会中止，绝不使用 InsecureIgnoreHostKey。

控制端解析全部 DNS 结果并检查地址，然后直接拨号到检查过的 IP，避免检查和连接之间的 DNS 重绑定。默认拒绝私有、回环、链路本地、保留地址、组播和云元数据/平台地址。部署者可通过 `XINGDU_SSH_ALLOWED_CIDRS` 允许指定私有管理网段；租户 API 无权修改此配置。回环、链路本地和已知元数据地址不随 allowlist 放行。仍建议部署者在出口防火墙层执行同样的网络隔离。

SSH 会话有连接和任务超时；检查受并发数和用户限流约束。远程执行仅有固定平台检查及安装命令。安装包通过加密 stdin 传送，不把密码、私钥、私钥口令、注册令牌放入命令参数；远端临时目录 0700、凭据文件 0600，退出时清理。硬中断无法保证远端临时目录已清理，令牌过期/撤销仍会阻止注册。

## 凭据存储与队列

配置 `XINGDU_CREDENTIAL_KEY`（64 位十六进制，即 256-bit 随机密钥）后才开放 SSH 安装。API 和 Worker 需要相同密钥；密钥必须存于部署环境或 secret manager，不能存于数据库、Git、日志。密钥丢失无法解密历史凭据；当前不提供自动密钥轮换，轮换前须规划重新加密或重新提交凭据。

- 默认：AES-256-GCM 加密存于任务，完成、失败、取消或过期后清除密文。
- 用户可选：另存长期加密凭据，可供本组织管理员再次安装，同样不会通过 API 回显。
- AAD 绑定组织、机器和用途/任务 ID；交换密文不能跨组织或跨机器解密。
- 密文中的连接目标与原始主机指纹一并校验。修改地址、端口或用户名后不能直接复用旧凭据。
- 「删除保存的 SSH 凭据」删除长期副本，已提交任务仍可使用自己的短期副本；需同时停止任务时使用撤销接入。
- 凭据保留在备份中的历史副本不因在线删除而消失，备份也应加密并设置保留期限。

安装任务持久化，Worker 使用独立 `xingdu_worker` 低权限登录角色。仅这个角色可通过固定 SECURITY DEFINER 函数领取全局任务 ID 与租户上下文，业务访问仍受 RLS；API 不能调用全局领取函数。Worker 每次执行与远程安装前重新检查发起者管理权限、任务租约及当前连接目标。

SSH 任务租约 3 分钟、操作超时 100 秒。过期的运行任务标记中断并清除密文，不自动重试，避免不确定的远程动作重复执行。排队超过 30 分钟过期；清理依赖 Worker 正常运行。用户需核实 VPS 状态后重新提交。

## 安装失败诊断

SSH 安装与升级会先检查 sh、systemctl 和 root 权限，再检查 tar。缺少 tar 时，
通过已有的 apt-get、dnf、yum 或 zypper 安装，仅安装 tar 及包管理器所需依赖，
不执行全系统升级，不添加软件源，不关闭包签名校验。已有 tar 时跳过包管理器。
安装后再次确认 tar 可执行，成功才上传安装包；任务仍受原有 SSH 超时限制。
不支持的包管理器返回 ssh_tar_missing，安装失败返回 ssh_tar_install_failed；
超时仍可能留下包管理器操作，需要核实 VPS 状态后再提交。
失败原因作为固定错误码保存在组织隔离的 `machine_jobs.result` 中，与任务 ID、
机器 ID、操作、创建和结束时间一并保留。页面显示错误说明和错误码，Worker
记录失败任务 ID、机器 ID、操作及错误码，方便定位和统计重复故障。

安装目录创建、解包、已有安装、状态目录、专用用户、systemd 重载和服务启动
使用各自的错误码。安装器错误只按已知固定消息映射；不保存原始远端输出，
不记录密码、私钥、注册令牌、机器地址或完整配置。未知错误仍使用通用错误码，
不能认为已经取得完整安装日志。历史失败任务不会被自动补写或重新安装。

2026-09-30 排障案例：AlmaLinux 8.9 缺少 tar，SSH 认证、root 权限、systemd、
安装目录权限和控制端 HTTPS 下载均正常，但解包步骤失败，尚未创建 Agent
状态目录或服务。当时修复方式为管理员在 VPS 执行 `dnf install -y tar`，再重新
提交安装。此前仅记录 `install_failed_check_vps`，随后增加了
`ssh_tar_missing` 前置诊断；当前版本进一步在权限检查后自动补装 tar，
补装失败时停止上传安装凭据。

自动化测试覆盖四种包管理器分支、已有 tar 时跳过、权限不足、安装失败与
安装后仍缺失；SSH 测试确认补装失败不会传送安装凭据。一次性 Amazon Linux
容器已实际移除 tar 并通过 dnf 自动补装成功；这不代表 AlmaLinux VPS 实机验收。

## 验收边界

自动化覆盖真实 PostgreSQL 的机器注册、心跳、离线推导、撤销、令牌重放/模式限制、队列角色权限、密文清除；实际 SSH 协议测试覆盖密码/PEM/加密私钥、指纹拒绝、安装包传送；此外测试加密篡改、AAD 隔离、SSRF 地址策略、构建和脚本语法。本地只读文件系统、无 capabilities 的低权限 Linux 容器已实测注册、连续心跳和撤销后自动退出。

目前 Compose 仍仅在本机开放。远程安装必须先部署机器可达、证书可信的 HTTPS 控制端，并设置 `XINGDU_AGENT_ORIGIN`。本地协议测试不等于在各发行版上完成 systemd 实机验收；生产发布前还需在专用 VPS 验证安装、重启恢复、断网恢复、撤销、失败恢复和卸载。

参考：[Go SSH API](https://pkg.go.dev/golang.org/x/crypto/ssh)、[systemd 执行权限](https://github.com/systemd/systemd/blob/main/man/systemd.exec.xml)、[AWS 元数据](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html)、[阿里云元数据](https://www.alibabacloud.com/help/en/ens/instance-metadata)、[Azure 平台地址](https://learn.microsoft.com/en-us/azure/virtual-network/what-is-ip-address-168-63-129-16)。

## Docker 发行版实验室

使用 `make agent-lab-up` / `make agent-lab-test` 在真实 Ubuntu、Debian、Amazon Linux 用户空间及 systemd/OpenSSH 环境中执行接入验收。隔离边界、清理和证据范围见 [AGENT-LAB.md](AGENT-LAB.md)。
