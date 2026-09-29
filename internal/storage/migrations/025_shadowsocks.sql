ALTER TABLE protocol_deployments DROP CONSTRAINT protocol_deployments_protocol_check;
ALTER TABLE protocol_deployments ADD CONSTRAINT protocol_deployments_protocol_check
 CHECK(protocol IN ('trojan','vless','vmess','hysteria2','tuic','shadowsocks','shadowsocks2022'));
