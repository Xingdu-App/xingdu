# Management API / 管理 API

API v1 supports organization-scoped automation for server inventory and managed
nodes. Open **API 密钥 / API keys** in the console sidebar
(`/app/api-keys?organization=org_...`). Only owners and administrators can
create, list or revoke keys. A key belongs to its issuing user and organization.

## Authentication and lifecycle

- Send `Authorization: Bearer <API_KEY>` over HTTPS. Do not send cookies,
  `Origin` or browser fetch metadata. Browser sessions retain their existing
  origin and CSRF checks.
- Keys contain 256 random bits; the full value is returned only on creation.
  The database stores SHA-256 hashes and a short display prefix.
- Expiration is required: 1–365 days; the UI defaults to 90 days. An organization
  can have up to 50 unexpired, unrevoked keys.
- Revoke a key in the console to disable subsequent requests. In-flight remote
  jobs already queued are not cancelled. Rotate by creating a replacement,
  updating your script, then revoking the old key.
- Membership removal or loss of owner/admin status prevents use. Authentication
  and each business transaction check validity under organization scope.
  Restoring the creator's admin role re-enables an otherwise valid key; use
  revocation for permanent invalidation.
- The key selects the organization. An optional `X-Xingdu-Organization` header
  must match it. API keys cannot manage keys, users, billing, subscriptions,
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
| POST /api/v1/hosts/{srv_id}/deployments/{node_id}/restart | nodes:write | Queue restart; JSON body `{"confirm":true}` |
| DELETE /api/v1/hosts/{srv_id}/deployments/{node_id} | nodes:write | Queue uninstall |
| POST /api/v1/hosts/{srv_id}/deployments/{node_id}/connection | nodes:credentials | Reveal client connection credentials; JSON body `{}` |

Use `Content-Type: application/json` for every mutation, including DELETE.
Responses use `{"data": ...}`; errors use
`{"error":{"code":"...","message":"..."}}`. Deletions may return HTTP 204
with no body. Protocol jobs return HTTP 202 when queued; this does **not**
mean installation or restart completed. Poll the deployments endpoint to
observe job state. There is no generic node-edit endpoint: replace deployments
using the existing uninstall/install flow.

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

Protocol values: `trojan`, `vless`, `vmess`, `hysteria2`, `tuic`.
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
