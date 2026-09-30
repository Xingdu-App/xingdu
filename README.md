<p align="center">
  <img src="apps/web/public/xingdu-logo.png" width="128" alt="Xingdu logo" />
</p>

# Xingdu · 星渡

**Your servers. Your routes. One place to manage them.**

[English](README.md) · [简体中文](README.zh-CN.md)

[![CI](https://github.com/Xingdu-App/xingdu/actions/workflows/ci.yml/badge.svg)](https://github.com/Xingdu-App/xingdu/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Xingdu is an open-source project building a multi-tenant SaaS control panel for VPS and proxy infrastructure, with self-hosting support. Its goal is to bring server onboarding, protocol deployment, direct and relay routes, and client subscriptions into one workflow.

Built for individuals and small teams, Xingdu is designed to work with different protocol engines and clients, without tying your infrastructure to a single client application.

> **Xingdu Cloud is now available at [xingdu.app](https://xingdu.app).** Use the hosted service or deploy the MIT-licensed edition yourself. Supported protocols, client exports and operational requirements are documented below. Verify your target server and client combination before relying on it; a successful local build does not establish compatibility or recovery readiness.

## What you can try today

- **Server inventory:** add, edit, and remove server records with addresses, SSH connection details, tags, and notes.
- **Team workspaces:** create and switch organizations, invite members, and assign owner, admin, member or viewer roles.
- **Tenant isolation:** PostgreSQL row-level security and a separate low-privilege runtime database account.
- **User sign-in:** verified email registration, email/password login, optional Google / GitHub sign-in and explicit account linking, local account provisioning, and revocable sessions. See [provider setup and verification limits](docs/SOCIAL-LOGIN.md).
- **Machine access:** manual Agent enrollment or SSH password/private-key installation, with pinned host keys and optional encrypted credential retention. See [machine access](docs/MACHINE-ACCESS.md) for setup and current validation limits.
- **Node inventory:** browse successfully deployed nodes across your organization, search and filter by protocol, and open node details. Nodes remain listed until uninstall succeeds; operation results stay in deployment records.
- **Protocol deployment and maintenance:** managed deployment of SS/SS2022, Trojan, VLESS, VMess, HY1/HY2, TUIC, AnyTLS, HTTPS, SOCKS5/Mixed, ShadowTLS and Snell options, with preflight, restart, configuration editing, credential rotation and revision recovery. See [protocol deployment](docs/PROTOCOL-DEPLOYMENT.md) and [reliable changes](docs/RELIABLE-DEPLOYMENTS.md) for transport, certificate, Agent version and validation limits.
- **Client subscriptions:** revocable Stash (default), Mihomo, Surge or Loon configuration links, or HY2 URIs, with custom rules, reusable templates and routing presets with multiple policy groups. Unsupported combinations are explicitly rejected. See [subscriptions](docs/SUBSCRIPTIONS.md) and [routing templates](docs/SUBSCRIPTION-TEMPLATES.md).
- **Live service status:** API and database availability, with error messages and retry when the connection fails.
- **Local Docker setup:** starts the web console, API, PostgreSQL, database migrations, and a worker process.
- **Development foundation:** Go and TypeScript code, automated checks, and separate worker and agent entry points.

The worker handles SSH bootstrap, and Agents poll for fixed, authorized tasks; arbitrary remote commands are not exposed. The console supports Simplified Chinese and English. Recorded container and selected real-machine checks include SS2022 on AlmaLinux 10.2 / amd64 / SELinux Enforcing and a Stash macOS 4.3.0 node test after policy repair. These results do not establish compatibility for every protocol, distribution or client version. See [implementation status](docs/IMPLEMENTATION-STATUS.md) for evidence and source versus released-artifact boundaries.

## Workflow and next steps

```text
Connect a VPS → Choose a protocol and route → Deploy and verify → Import into your client
```

Configuration revisions, failure recovery, single TCP relays and an operator-run
probe process are implemented. Automatic certificates use an operator-run
ACME DNS-01 / Cloudflare process; tenant self-service authorization is not available.
Next steps focus on real client combinations, public-network recovery and
certificate renewal acceptance, plus Passkey, notifications and more transports.
See [implementation status](docs/IMPLEMENTATION-STATUS.md) and the
[development plan](docs/PLAN.md).

## Try it locally

You need **Git, Docker, and Docker Compose v2**. Go and Node.js are not required for the container-based setup.

```sh
git clone https://github.com/Xingdu-App/xingdu.git
cd xingdu
cp .env.example .env
docker compose up --build -d --wait
```

Open the public website at **[http://127.0.0.1:15173](http://127.0.0.1:15173)** and the console at **[/app](http://127.0.0.1:15173/app)**. The website includes pricing, privacy and security pages; hosted plans are Starter, Premium and Enterprise; see [billing](docs/BILLING.md). The first build downloads dependencies and container images, so it may take a few minutes.

Create your initial user and organization from a local terminal. The command prompts for a hidden password of 12–72 bytes; there is no default password. It refuses to overwrite an existing username.

```sh
docker compose exec api admin --username admin
```

Sign in, select an organization, then add your first server record. In **组织与成员**, generate a single-use invitation link valid for seven days; share it privately with the intended member. Set `XINGDU_REGISTRATION_ENABLED=true` to enable sign-up; otherwise provision users through the same CLI. Saving connection details does not contact the VPS: new records remain **Pending enrollment** until a registered Agent reports a real heartbeat. Do not put passwords or private keys in notes.

| Service | Local address |
| --- | --- |
| Public website | `http://127.0.0.1:15173` |
| Web console | `http://127.0.0.1:15173/app` |
| API | `http://127.0.0.1:18080` |
| PostgreSQL | `127.0.0.1:54329` |

All published ports bind to loopback. Use the exact console URL above: write requests are checked against `XINGDU_PUBLIC_ORIGIN`, which defaults to `http://127.0.0.1:15173`. The example credentials are for local development only. If you already have a `.env` file, keep it instead of copying over it, and add `XINGDU_APP_DATABASE_PASSWORD` (at least 16 URL-safe characters) for the separate runtime account, plus `XINGDU_WORKER_DATABASE_PASSWORD` for the installation worker.

To stop the preview while keeping database data:

```sh
docker compose down
```

## Development and contributions

Want to help build Xingdu? Bug reports, use cases, documentation improvements, and code contributions are welcome. For larger changes, open an [issue](https://github.com/Xingdu-App/xingdu/issues) first to discuss the scope.

For development outside containers, install the Go version specified in `go.mod`, Node.js 24 LTS, and Make. Then run:

```sh
make setup
make check
```

The [development guide](docs/DEVELOPMENT.md) covers hot reload, database setup, and integration tests. Database integration tests require a dedicated test database; they are skipped unless `XINGDU_TEST_DATABASE_URL` is set.

| Resource | Contents |
| --- | --- |
| [Implementation status](docs/IMPLEMENTATION-STATUS.md) | Current capabilities, releases and acceptance evidence |
| [Management API](docs/API.md) | Organization API keys, scopes and server/node automation |
| [Protocol deployment](docs/PROTOCOL-DEPLOYMENT.md) | Supported protocols, TLS, Agent requirements and lifecycle |
| [SaaS architecture](docs/SAAS.md) | Tenant boundaries, roles, invitations, and RLS |
| [Development guide](docs/DEVELOPMENT.md) | Local setup, commands, and API endpoints |
| [Development plan](docs/PLAN.md) | Scope and milestones |
| [Adapter design](docs/ADAPTERS.md) | Runtime and client integration boundaries |
| [Contributing](CONTRIBUTING.md) | Contribution guidelines |
| [Security](SECURITY.md) | Vulnerability reporting guidance |

Supporting documents are currently in Simplified Chinese. Do not include credentials, private server details, or subscription tokens in public issues.

## License

Xingdu is licensed under the [MIT License](LICENSE). Personal and commercial use, modification, and redistribution are permitted under its terms. Retain the copyright and license notices when distributing copies or substantial portions of the software.

Third-party dependencies and protocol engines remain subject to their own licenses. The current runtime is the separately executed sing-box 1.14.2, licensed by its upstream project under GPL-3.0-or-later; Xingdu’s MIT license does not relicense that runtime. See [third-party notices](THIRD_PARTY_NOTICES.md).
