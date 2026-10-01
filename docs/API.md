# Management API / 管理 API

API v1 supports organization-scoped automation for server inventory and managed
nodes. Open **API 密钥 / API keys** in the console sidebar
(`/app/api-keys?organization=org_...`). Only owners and administrators can
create, list, edit scopes or revoke keys. The dedicated console reference is available at
`/app/api-docs`. A key belongs to its issuing user and organization.

## Authentication and lifecycle

- Send `Authorization: Bearer <API_KEY>` over HTTPS. Do not send cookies,
  `Origin` or browser fetch metadata. Browser sessions retain their existing
  origin and CSRF checks.
- Keys contain 256 random bits; the full value is returned only on creation.
  The database stores SHA-256 hashes and a short display prefix.
- Expiration is required: 1–365 days; the UI defaults to 90 days. An organization
  can have up to 50 unexpired, unrevoked keys.
- Edit an active key's scopes in the console without rotating its secret. The
  browser-only `PATCH /api/v1/api-keys/{id}` accepts `{"scopes":["hosts:read"]}`
  and returns key metadata without the secret or hash. Owner/admin access,
  organization scope and CSRF validation are required; bearer keys cannot
  call this endpoint. Empty, duplicate or unknown scopes are rejected, and
  expired or revoked keys cannot be edited. Changes apply to subsequent
  requests; already queued jobs are not cancelled.
- Revoke a key in the console to disable subsequent requests. In-flight remote
  jobs already queued are not cancelled. Rotate by creating a replacement,
  updating your script, then revoking the old key.
- Membership removal or loss of owner/admin status prevents use. Authentication
  and each business transaction check validity under organization scope.
  Restoring the creator's admin role re-enables an otherwise valid key; use
  revocation for permanent invalidation.
- The key selects the organization. An optional `X-Xingdu-Organization` header
  must match it. API keys cannot manage keys, users, billing,
  SSH credentials or Agent identities.
- Limit: 120 authorized requests per key per minute **per API process**, not a
  distributed quota. HTTP 429 includes `Retry-After: 60`.
- Revocation metadata records who revoked the key and when. Recent-use time
  records successful authentication, including requests subsequently rejected
  for insufficient scope. It is not a per-request audit log.

## Scopes and endpoints

Scopes are independent: a write scope does not imply read or credential access.

| Method and path | Required scope | Behavior |
| --- | --- | --- |
| GET /api/v1/hosts | hosts:read | List server inventory |
| POST /api/v1/hosts | hosts:write | Create server inventory |
| PUT /api/v1/hosts/{srv_id} | hosts:write | Replace editable server metadata |
| DELETE /api/v1/hosts/{srv_id} | hosts:write | Delete server inventory, subject to existing safety checks |
| GET /api/v1/nodes | nodes:read | List nodes |
| GET /api/v1/hosts/{srv_id}/deployments | nodes:read | List server protocol deployments |
| POST /api/v1/hosts/{srv_id}/deployments/preflight | nodes:write | Run installation preflight |
| POST /api/v1/hosts/{srv_id}/deployments | nodes:write | Queue installation |
| PUT /api/v1/hosts/{srv_id}/deployments/{node_id} | nodes:write | Queue configuration update; requires `confirm: true` |
| POST /api/v1/hosts/{srv_id}/deployments/{node_id}/probe | nodes:probe | Report a client probe result; does not initiate a probe |
| POST /api/v1/hosts/{srv_id}/deployments/{node_id}/restart | nodes:write | Queue restart; JSON body `{"confirm":true}` |
| DELETE /api/v1/hosts/{srv_id}/deployments/{node_id} | nodes:write | Queue uninstall |
| POST /api/v1/hosts/{srv_id}/deployments/{node_id}/connection | nodes:credentials | Reveal client connection credentials; JSON body `{}` |

Use `Content-Type: application/json` for every mutation, including DELETE.
Responses use `{"data": ...}`; errors use
`{"error":{"code":"...","message":"..."}}`. Deletions may return HTTP 204
with no body. Protocol jobs return HTTP 202 when queued; this does **not**
mean installation or restart completed. Poll the deployments endpoint to
observe job state. Configuration updates use PUT on the deployment with `confirm: true`, for
example `{"port":8443,"confirm":true}`. Probe reports use
`{"ok":true,"latency_ms":120,"exit_ip":"203.0.113.10","revision":1}`;
replace the example IP and revision with observed values. Probe reports return
`{"ok":true}` rather than a `data` envelope.

Server request body:

```json
{
  "name": "Automation server",
  "address": "vps.example.com",
  "ssh_port": 22,
  "ssh_user": "root",
  "tags": ["automation"],
  "notes": ""
}
```

Creating inventory does not enroll an Agent or connect by SSH. A managed Agent
must already be connected before protocol installation. See
[MACHINE-ACCESS.md](MACHINE-ACCESS.md) and
[PROTOCOL-DEPLOYMENT.md](PROTOCOL-DEPLOYMENT.md) for supported protocol inputs
and runtime restrictions. Installation additionally requires
`confirm_install: true`.

For preflight and installation, send the following fields. Replace the
certificate placeholders with complete PEM contents (JSON-escaped newlines).
Preflight uses the same body without `confirm_install`.

```json
{
  "name": "Automation node",
  "protocol": "trojan",
  "port": 443,
  "server_name": "node.example.com",
  "certificate": "<PEM certificate chain>",
  "private_key": "<PEM private key>",
  "confirm_install": true
}
```

Protocol values: `shadowsocks`, `shadowsocks2022`, `trojan`, `vless`,
`vmess`, `hysteria2`, `tuic`, `anytls`, `http`, `socks`, `mixed`, `hysteria`,
`shadowtls`, `snell`, `snell6`. The last six require Agent 0.14.0-dev.
Shadowsocks, SOCKS5, Mixed and Snell omit TLS fields. `http` is an HTTPS proxy
and requires a certificate; `mixed` supports plain HTTP and SOCKS5 on one port.
Hysteria 1 also requires a certificate and uses fixed 100 Mbps bandwidth hints.
ShadowTLS requires an approved `server_name` (`www.microsoft.com`,
`www.apple.com` or `cloud.tencent.com`) with no certificate/private key.
Its connection response contains the SS2022 key in `credential` and a separate
ShadowTLS v3 secret in `password`. `snell` exports client version 4 for the
runtime's v5 server without QUIC proxy mode; `snell6` is the v6 beta mode.
See [deployment prerequisites](PROTOCOL-DEPLOYMENT.md) and the explicit
[client format matrix](SUBSCRIPTIONS.md).
Do not blindly retry installation after a timeout: check the deployment list
first, because a queued job might have been accepted before the response was
lost.

## Example (Python standard library)

This example prompts for the secret without putting it in shell history or
process arguments. For unattended scripts, load the key from your secret
manager. Never embed it in a browser application, URL, repository or log.

```python
import getpass
import json
import urllib.request

base = "https://xingdu.app"
key = getpass.getpass("Xingdu API key: ")

def request(method, path, payload=None):
    body = None if method == "GET" else json.dumps(payload or {}).encode()
    req = urllib.request.Request(
        base + "/api/v1" + path,
        data=body,
        method=method,
        headers={
            "Authorization": "Bearer " + key,
            "Content-Type": "application/json",
        },
    )
    with urllib.request.urlopen(req, timeout=30) as response:
        return None if response.status == 204 else json.load(response)["data"]

for server in request("GET", "/hosts"):
    print(server["id"], server["name"])
for node in request("GET", "/nodes"):
    print(node["id"])
```

The example requires `hosts:read` and `nodes:read`. Existing inventory,
organization quotas, pinned SSH and protocol safety restrictions remain in
effect. This API is an early preview; local tests do not establish real VPS or
client compatibility.

## Subscription configuration API

New keys can explicitly request `subscriptions:read`, `subscriptions:write`
and `subscriptions:export`. Existing keys retain their original scopes.
The export scope authorizes **node connection credentials** in generated files
and previews; read/write scopes alone do not grant export access. All operations
retain organization RLS and owner/admin restrictions for changes and exports.
API keys cannot rotate or reveal bearer subscription links.

| Method and path | Scope | Result |
| --- | --- | --- |
| GET /api/v1/subscriptions | subscriptions:read | Settings list without bearer links or node credentials |
| GET /api/v1/subscriptions/{sub_id} | subscriptions:read | Settings and `ETag` |
| POST /api/v1/subscriptions | subscriptions:write | Create; key responses omit the bearer link |
| PATCH /api/v1/subscriptions/{sub_id} | subscriptions:write | Apply only supplied fields; requires `If-Match` |
| PUT /api/v1/subscriptions/{sub_id} | subscriptions:write | Replace editable settings; API keys require `If-Match` |
| DELETE /api/v1/subscriptions/{sub_id} | subscriptions:write | Delete the subscription |
| GET /api/v1/subscriptions/{sub_id}/config?format=stash | subscriptions:export | Download a generated configuration |
| POST /api/v1/subscriptions/{sub_id}/preview | subscriptions:export | Preview a patch without saving settings |

### Read, patch, preview and download

Read a subscription before modifying it. The response includes `revision: 3`
and an `ETag: "3"` header. Send that exact header value as `If-Match: "3"`.
PATCH without it returns 428; malformed values return 400. A stale version or
unavailable selected node returns 409. Re-read and reconcile changes rather than
blindly retrying with a newer version. Revision checks happen inside the same
transaction as the write. Link rotation also advances the settings revision.
Browser full replacements can carry the `revision` JSON field; the updated
console does so. Legacy browser PUT without a revision remains supported and
has no conflict protection. Prefer PATCH for new integrations.

PATCH and preview accept `name`, `format`, `node_ids`, `rules`, `final_action`,
`enabled`, `routing` and `group_updates`. Omitted fields retain their values.
Arrays replace the supplied collection; send `[]` to clear it. `routing: null`
removes the routing template. Arbitrary YAML/CONF uploads are not supported.
Unknown fields are rejected. Node/group/rule/protocol validation still applies.

For example, change only one existing group's icon:

```http
PATCH /api/v1/subscriptions/{sub_id}
Authorization: Bearer <API_KEY>
Content-Type: application/json
If-Match: "3"
```

```json
{
  "group_updates": [
    {"id": "proxy", "icon": "https://assets.example.com/icons/proxy.png"}
  ]
}
```

Group IDs are stable identifiers, not display names. `icon: null` (or `""`)
clears the icon. Unknown or duplicate group IDs are rejected. Do not combine
`routing` replacement and `group_updates` in one request. A successful mutation
returns the updated settings and new ETag; ordinary edits preserve the existing
client subscription URL. Clients obtain the new config when refreshing it.

POST the same patch body to `/preview`. The JSON `data` contains `format`,
`revision`, `content` and a `warnings` array. This does not save the draft,
advance its revision, rotate its token or run a remote deployment. Credential
access is audited. Preview and download contain real connection credentials
and use `Cache-Control: no-store`; keep their output private.

Download uses the saved default format unless `?format=` is supplied. It returns
YAML for Stash/Mihomo or text for Surge/Loon/Hysteria 2 URI, with an attachment
filename and `X-Xingdu-Subscription-Revision`. That header identifies settings,
not a complete cache validator: node credentials and configs can change
independently. Disabled subscriptions can still be exported by an authorized
manager; their public bearer link remains disabled. Any unavailable selected
node fails the managed export rather than silently producing a partial file.

### Icon compatibility

Only HTTPS URLs without embedded credentials or fragments are accepted, up to
2048 characters. Localhost/private literal IPs and control characters are
rejected. The API stores URLs and never fetches the images. Use a public image
host; clients fetch images themselves, so host privacy and availability matter.

- Stash YAML emits `proxy-groups[].icon`; JPG/PNG are documented by the client.
- Mihomo YAML emits the same field; display depends on the client/dashboard.
- Surge, Loon and URI formats do not export group icons in this implementation.
  Preview includes `group_icons_not_exported_for_format`; downloads include it
  in `X-Xingdu-Config-Warnings`. Mihomo reports
  `icon_display_depends_on_dashboard`.

See [Stash icon documentation](https://stash.wiki/en/configuration/proxy-group-icon)
and [Mihomo proxy groups](https://wiki.metacubex.one/en/config/proxy-groups/).
These are format mappings, not a record of real-app display acceptance.

## 托管证书

需要已登录浏览器组织会话，或 `certificates:read` / `certificates:write` API Key。
所有写入需要组织所有者/管理员；创建、签发和节点应用还需要有效付费订阅。
缺失运营配置返回 `503 certificate_provider_unavailable`；免费/过期套餐返回
`403 paid_subscription_required`；超额返回 `409 certificate_quota_exceeded`。

| 方法与路径 | 用途 | Key scope |
| --- | --- | --- |
| `GET /api/v1/certificates` | 证书元数据、配额、配置/测试状态 | `certificates:read` |
| `POST /api/v1/certificates` | 自有域名 `{ "domain": "node.example.com" }`；或随机平台域名 `{ "platform": true, "host_id": "srv_…" }` | `certificates:write` |
| `POST /api/v1/certificates/{id}/issue` | DNS 验证及签发/续期排队；正常续期要求到期不足 30 天，切换 CA 可重签 | `certificates:write` |
| `DELETE /api/v1/certificates/{id}` | 停止管理、删除托管密文，保留域名墓碑 | `certificates:write` |
| `POST /api/v1/certificates/{id}/apply` | `{ "host_id": "srv_…", "node_id": "node_…", "confirm": true }`，更新 TLS 域名及证书并排队 Agent 任务 | 仅浏览器组织会话 |

新建 TLS 节点时，`POST /api/v1/hosts/{srv_id}/deployments` 可提交
`certificate_id`，例如
`{ "name": "Tokyo Trojan", "protocol": "trojan", "port": 8443,
"certificate_id": "cert_…", "confirm_install": true }`。
此方式要求 API Key 同时具有 `nodes:write` 和 `certificates:write`，
服务端注入证书与私钥，并自动使用证书域名作为 `server_name`。
不要同时提交 PEM；只接受有效正式证书，平台域名必须绑定目标机器。
部署排队事务会再次校验证书、付费资格及配额；私钥不会出现在响应中。

列表包含 `id/domain/validation_target/platform/host_id/state/expires_at/error_code/directory`，
绝不包含 PEM 私钥、证书密文或 CA 账户。`directory` 用于识别正式/测试 CA。
平台域名仅应用到绑定机器，测试 CA 证书不能应用。`202` 仅表示排队，最终结果
见节点部署记录。续期不会自动重启节点，移除管理也不撤销已安装证书。

### Protocol matrix parameters

Deployment create/preflight/update accept bounded `v2ray`, `quic`, `wireguard`
and SOCKS `udp_enabled` settings. Connection reveal returns client parameters
and generated client credentials; encrypted revisions retain server secrets. See [the full contract, minimum
Agent version, examples and restrictions](TRANSPORT-COMPATIBILITY.md).
