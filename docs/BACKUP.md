# 加密备份与恢复

`go build -trimpath -o bin/xingdu-backup ./cmd/backup` 构建离线运维 CLI。需要与服务器主版本匹配（或兼容的更新版本）的 `pg_dump` / `pg_restore`。这不是面向普通组织管理员的 Web 下载接口。

## 密钥和权限

备份使用独立的 32 字节密钥，通过 `XINGDU_BACKUP_KEY` 环境变量提供 Base64。可以用 `bin/xingdu-backup keygen` 生成，直接保存至受控密钥管理器，避免把输出复制到日志或聊天中。不要将此密钥加入 `.env` 示例、版本库或命令行参数。

PostgreSQL 连接使用 `PGHOST`、`PGPORT`、`PGUSER`、`PGDATABASE`、`PGSSLMODE`，凭据使用 `PGPASSFILE`（0600）或受控注入的 `PGPASSWORD`。`PGDATABASE` 必须是普通库名，不能放连接 URI。生产连接应验证服务器证书。

备份角色必须能够读取完整数据库，包括 RLS 表；通常由受控的数据库所有者或备份运维角色执行。**不要给应用运行角色 `xingdu_app` BYPASSRLS，也不要把备份凭据提供给 API/Worker。** `pg_dump` 遇到无法完整读取的 RLS 数据会失败，不会用租户过滤后的部分数据伪装完整备份。

备份加密密钥、应用的凭据加密主密钥（以及将来轮换所需的旧密钥）、数据库备份应分别保存。数据库内的机器/节点密文依赖应用主密钥，只有数据库备份不足以恢复业务。环境配置、反向代理证书、Agent enrollment 配置和文件型密钥也应按各自敏感性另行备份；这个 CLI 只备份 PostgreSQL。

## 创建

在安全注入上述环境变量后：

```sh
bin/xingdu-backup create --file /secure-backups/xingdu-2026-09-28.xdb
```

输出文件权限为 0600，不覆盖已有文件或符号链接。先写同目录临时文件，在 `pg_dump` 成功且文件同步后以不覆盖方式发布。失败会删除临时文件。完整配置、数据、ACL 和 RLS 策略保存在 PostgreSQL custom archive 中；保留对象原 owner，包括 SECURITY DEFINER 函数的专用所有者；不会使用 `--no-owner` 或将这些函数重新归属为高权限恢复账户。

格式 v1 使用每个归档独立的 256 位随机 salt、HKDF-SHA256 派生 AES-256-GCM 密钥和有序记录。头部、记录长度、序号及必须存在的结束记录都受到认证；截断、重排、篡改、错误密钥和追加内容均拒绝。数据按 1 MiB 记录处理，不需要把整个数据库放入内存。

## 恢复到新库

1. 停止目标环境的应用写入；创建一个**新的空数据库**作为恢复目标。保留原数据库，避免直接覆盖。
2. 在新 PostgreSQL 实例上先按已部署版本的迁移文件建立全部专用角色和原数据库对象所有者。当前包括 `xingdu_app`、`xingdu_worker`、`xingdu_policy`、`xingdu_ownership`、`xingdu_quota` 以及原 schema/迁移所有者；未来新增角色也必须同步。app/worker/ownership 为 `NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE`；仅 policy 角色按照原迁移为 `NOLOGIN NOSUPERUSER BYPASSRLS NOCREATEDB NOCREATEROLE`，只用于受限 SECURITY DEFINER 函数，不可赋予应用登录身份。配置原角色成员关系。数据库级角色/密码不由 `pg_dump` 保存；恢复身份需具备恢复这些 owner 和 ACL 的权限。不得以忽略 owner/ACL 的选项绕过失败。
3. 用迁移所有者的连接环境明确设置 `PGDATABASE`，并注入备份密钥。执行：

```sh
bin/xingdu-backup restore --file /secure-backups/xingdu-2026-09-28.xdb --confirm-database xingdu_restore
```

`--confirm-database` 必须与 `PGDATABASE` 和服务端 `current_database()` 一致。存在用户表、函数、类型或额外 schema 的数据库会拒绝恢复。先将整个归档认证解密到 0600 临时文件，认证完成之前不会连接目标数据库；`pg_restore` 在单事务内执行，SQL 失败回滚。完成后删除明文临时文件。请使用加密磁盘；操作系统崩溃或 SIGKILL 可能留下临时文件，由运维恢复后清理 `xingdu-restore-*.dump`。恢复目标必须由运维独占，检查与恢复期间不能有其他进程创建对象或连接使用该库。

4. 恢复应用加密主密钥，核对 owner/ACL/RLS 与应用非 owner 连接角色，运行迁移、组织隔离检查、节点凭据解密、订阅读取测试，再切换应用数据库连接。
5. 恢复旧快照可能重新激活旧会话、邀请或订阅令牌；结合快照之后的撤销记录处理，再开放公网。Agent 的机器端状态不会随数据库回滚，先核对运行状态和待执行任务。

只持有解密后的归档也能读取敏感数据，应视为机密。不要为了排错输出 pg_dump/pg_restore 的完整 stderr 或明文归档。

## 已验证与后续

单测覆盖多记录/空归档往返、随机 salt、篡改、截断、结束记录缺失、错误密钥、追加内容和重排。`cmd/backup` 集成测试在指定测试 PostgreSQL 实例中创建两个一次性数据库，备份并恢复数据，验证 ACL/RLS、SECURITY DEFINER 专用 owner 未变为高权限身份以及拒绝覆盖非空库，最后删除两个测试库。仅使用专用测试实例：

```sh
XINGDU_TEST_BACKUP_DATABASE_URL="$DISPOSABLE_TEST_DATABASE_URL" go test ./internal/backup ./cmd/backup
```

测试账户需要 CREATEDB/CREATEROLE，并安装匹配的 PostgreSQL 客户端；测试会建立并清理专用 NOLOGIN/NOBYPASSRLS 角色，不依赖已有业务角色。2026-09-28 本地一次性 PostgreSQL 往返验收通过。尚未验证真实生产全库恢复、异地对象存储、备份定时调度、保留策略、密钥灾难恢复；这些不能由一次小型数据库测试替代。生产应定期在隔离环境演练并记录 RPO/RTO。
