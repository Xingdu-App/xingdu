# Resource identifiers

Xingdu resource IDs are opaque strings made of a type prefix, `_`, and exactly **32 lowercase hexadecimal characters**. New IDs contain 16 cryptographically random bytes (128 random bits). They do not encode creation time, user information, sequence numbers, or organization membership.

| Prefix | Resource |
|---|---|
| `usr_` | User |
| `org_` | Organization |
| `srv_` | Server / host |
| `node_` | Managed protocol deployment / node |
| `op_` | Deployment operation |
| `lease_` | Worker / Agent task lease identity |
| `ses_` | Browser login session record |
| `inv_` | Organization invitation record |
| `job_` | Machine bootstrap job |
| `sub_` | Subscription |
| `bat_` | Billing checkout attempt |

For example, `org_1234567890abcdef1234567890abcdef` is an organization-shaped ID and `node_fedcba0987654321fedcba0987654321` is a node-shaped ID. These are documentation placeholders, not live resources. Each endpoint must validate its expected resource type. An ID's prefix is not authorization: login, organization membership and PostgreSQL RLS still determine access. Tokens, token hashes, OAuth state and numeric internal audit counters have their own formats and are not resource IDs.

## Frontend and API

Keep the entire ID as a string in JSON, path parameters, navigation state and query parameters. Do not strip the prefix, cast it to a number, or pass it through UUID validators. The `X-Xingdu-Organization` header carries a complete `org_` value.

Example detail link:

```text
/app/nodes?organization=org_1234567890abcdef1234567890abcdef&node=node_fedcba0987654321fedcba0987654321
```

Resource IDs are database identities, not bearer secrets. A subscription's `sub_` ID is separate from its secret link token. Session `ses_` IDs displayed in Security are separate from session cookies. Invitation `inv_` IDs are separate from invitation bearer tokens. Knowing a resource ID does not grant access.

## Existing records and rollout

The controlled database migration maps an old resource UUID to its type prefix plus the same 32 lowercase hex digits without hyphens. For example, a server UUID `12345678-90ab-cdef-1234-567890abcdef` becomes `srv_1234567890abcdef1234567890abcdef`. This is deterministic so existing data and references can be migrated together. Existing records retain the entropy of their previous identifiers; the 128-random-bit guarantee applies to newly generated IDs.

UUIDs are no longer public resource identifiers after migration. Old deep links and external integrations need the new complete IDs; the public API does not silently accept arbitrary legacy UUID input. Migration code may recognize old IDs only inside an explicit migration boundary.

Before rollout, back up the database and encryption keys. Update database keys, foreign keys, RLS helpers, encrypted-field associated data and Agent compatibility as one coordinated migration. Agent 0.7 keeps existing UUID-named directories and systemd units as private filesystem compatibility details, resolving the new node ID deterministically; new installations use prefixed names. It does not rename a running service directory. An ID-only SQL rewrite is insufficient for values that bind ciphertext or machine-side service identity. Verify that existing credentials decrypt, Agents reconnect, existing nodes remain manageable, and subscription tokens still select the intended records. Stop API and Worker traffic before the migration; the migration also locks public tables before resealing ciphertext. Supply the existing `XINGDU_CREDENTIAL_KEY` to the migration job. Missing or incorrect keys abort without changing IDs. OAuth attempts in progress are invalidated and must be restarted. Upgrade Agents to 0.7 before resuming deployment work; earlier versions cannot claim new tasks. Stripe customer/subscription/checkout identifiers are external provider IDs and are never rewritten. If a deployment has used Stripe, reconcile pending or uncertain provider requests before this breaking migration; existing remote checkout return URLs cannot be rewritten by a database migration.

The existence of this design document does not establish that a target environment completed those checks.

Local lab scripts read API IDs for new resources. When reading pre-migration `.local/agent-lab/fixtures.json`, they resolve old server references in memory using the deterministic mapping and require an exact API ID plus the expected disposable address and `agent-lab` tag. They never guess another server from its name alone. This lookup does not rewrite the fixture file or migrate real machine-side state. Normal explicit lab fixture creation/cleanup remains separate.

## Protocol UUID credentials remain unchanged

VLESS, VMess and TUIC v5 define UUID authentication fields in their protocols. Those UUIDs are **connection credentials**, not Xingdu resource IDs, and must remain valid protocol UUIDs. A node can therefore have a `node_...` resource ID and a separate UUID credential at the same time. Never transform the latter to `node_`, `usr_`, or another resource prefix. TLS fingerprints, keys and authentication passwords also remain unchanged.

## Validation

- Go ID validation and API/storage tests cover type prefixes, shape and access boundaries.
- Subscription tests reject UUID/wrong-prefix node resource IDs while retaining valid VLESS UUID credentials.
- Frontend navigation tests round-trip prefixed organization/server/node IDs without truncation or reinterpretation.
- `python3 -m unittest discover -s scripts -p 'test_*.py'` checks migration-only local fixture mapping, rejection of unsafe/wrong-namespace values, target metadata verification, and non-mutation of the original fixture dictionary. It performs no network or machine operations.
