# Xingdu Agent 0.18.1

## English

Snell v6 nodes can now forward UDP when enabled in their configuration. Update
Agent first, then enable UDP in the node editor and refresh your client
subscription. Compatible Stash and Surge exports reflect the selected setting.
Existing nodes keep their current setting until explicitly updated.

TCP relay routes remain TCP-only. Older services without the required private
network protection cannot enable UDP through a configuration update and must be
recreated. This release does not add support for older Snell clients.

## 简体中文

Snell v6 节点现在可以在配置中启用 UDP 转发。请先升级 Agent，再在节点编辑中
开启 UDP，并刷新客户端订阅。兼容的 Stash 和 Surge 订阅会同步输出所选设置。
已有节点在主动更新前保持原来的设置。

TCP 中转线路仍仅支持 TCP。缺少必要私网保护策略的旧服务不能通过配置更新
启用 UDP，需要重新创建节点。本次更新不增加对旧版 Snell 客户端的支持。

## 繁體中文

Snell v6 節點現在可以在設定中啟用 UDP 轉發。請先升級 Agent，再於節點編輯中
開啟 UDP，並重新整理用戶端訂閱。相容的 Stash 和 Surge 訂閱會同步輸出所選設定。
既有節點在主動更新前維持原來的設定。

TCP 中轉線路仍僅支援 TCP。缺少必要私有網路保護策略的舊服務無法透過設定更新
啟用 UDP，需要重新建立節點。本次更新不增加對舊版 Snell 用戶端的支援。
