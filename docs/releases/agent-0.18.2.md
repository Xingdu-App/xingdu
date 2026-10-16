# Xingdu Agent 0.18.2

## English

Shadowsocks and Shadowsocks 2022 nodes can now forward UDP. Upgrade Agent,
enable UDP in the node editor, allow the node UDP port in your firewall, and
refresh your client subscription. Stash, Mihomo, Surge and sing-box exports
reflect this setting. Existing nodes keep UDP disabled until explicitly enabled.

TCP relay routes and ShadowTLS remain TCP-only. Services without the required
private network protection must be recreated before enabling UDP. An occupied
UDP port prevents the update without changing the running service.

## 简体中文

Shadowsocks 和 Shadowsocks 2022 节点现在可以转发 UDP。请升级 Agent，在节点
编辑中开启 UDP，在防火墙放行节点 UDP 端口，再刷新客户端订阅。Stash、Mihomo、
Surge 和 sing-box 订阅会同步此设置。已有节点需主动开启 UDP。

TCP 中转线路和 ShadowTLS 仍仅支持 TCP。缺少必要私网保护的旧服务需重新创建
才能开启 UDP。若 UDP 端口被占用，更新会被拒绝，运行中的服务保持不变。

## 繁體中文

Shadowsocks 和 Shadowsocks 2022 節點現在可以轉發 UDP。請升級 Agent，在節點
編輯中開啟 UDP，於防火牆放行節點 UDP 連接埠，再重新整理用戶端訂閱。Stash、
Mihomo、Surge 和 sing-box 訂閱會同步此設定。既有節點需主動開啟 UDP。

TCP 中轉線路和 ShadowTLS 仍僅支援 TCP。缺少必要私有網路保護的舊服務需重新
建立才能開啟 UDP。若 UDP 連接埠被占用，更新會被拒絕，執行中的服務保持不變。
