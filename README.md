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

> **Early development preview.** You can run the console locally today. Multi-user sign-in, organizations, member roles, invitation links, and isolated server inventories are available. Agent enrollment, telemetry, and SSH installation flows are implemented; protocol deployment and subscription export are not yet available. Keep this preview local; it is not ready for public or production deployment.

## What you can try today

- **Server inventory:** add, edit, and remove server records with addresses, SSH connection details, tags, and notes.
- **Team workspaces:** create and switch organizations, invite members, and assign owner, admin, member or viewer roles.
- **Tenant isolation:** PostgreSQL row-level security and a separate low-privilege runtime database account.
- **User sign-in:** optional registration, revocable sessions, and protected management endpoints.
- **Machine access:** manual Agent enrollment or SSH password/private-key installation, with pinned host keys and optional encrypted credential retention. See [machine access](docs/MACHINE-ACCESS.md) for setup and current validation limits.
- **Live service status:** API and database availability, with error messages and retry when the connection fails.
- **Local Docker setup:** starts the web console, API, PostgreSQL, database migrations, and a worker process.
- **Development foundation:** Go and TypeScript code, automated checks, and separate worker and agent entry points.

The worker executes SSH Agent installation jobs. The agent reports machine status; protocol deployment and arbitrary remote commands are not implemented. The [Docker lab](docs/AGENT-LAB.md) verifies systemd installation on Ubuntu 24.04, Debian 13 and Amazon Linux 2023 (arm64); real VPS/EC2 acceptance testing is still required. The current console UI is in Simplified Chinese.

## Where Xingdu is heading

```text
Connect a VPS → Choose a protocol and route → Deploy and verify → Import into your client
```

| Area | Planned capabilities |
| --- | --- |
| Server management | Onboard existing Linux VPS instances and track their health |
| Protocol deployment | Generate and validate configurations through runtime adapters |
| Route management | Direct connections and single-relay routes |
| Reliable changes | Versioned deployments, progress tracking, retries, and rollback |
| Client subscriptions | Dedicated export adapters for Stash, Surge, Loon, and Shadowrocket |

These are roadmap items, not supported features in the current preview. Protocol and client-version compatibility will be documented as combinations are tested. See the [development plan](docs/PLAN.md) for milestones.

## Try it locally

You need **Git, Docker, and Docker Compose v2**. Go and Node.js are not required for the container-based preview.

```sh
git clone https://github.com/Xingdu-App/xingdu.git
cd xingdu
cp .env.example .env
docker compose up --build -d --wait
```

Open the public website at **[http://127.0.0.1:15173](http://127.0.0.1:15173)** and the console at **[/app](http://127.0.0.1:15173/app)**. The website includes pricing, privacy and security pages; hosted service pricing is not yet announced. The first build downloads dependencies and container images, so it may take a few minutes.

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
| [SaaS architecture](docs/SAAS.md) | Tenant boundaries, roles, invitations, and RLS |
| [Development guide](docs/DEVELOPMENT.md) | Local setup, commands, and API endpoints |
| [Development plan](docs/PLAN.md) | Scope and milestones |
| [Adapter design](docs/ADAPTERS.md) | Runtime and client integration boundaries |
| [Contributing](CONTRIBUTING.md) | Contribution guidelines |
| [Security](SECURITY.md) | Vulnerability reporting guidance |

Supporting documents are currently in Simplified Chinese. Do not include credentials, private server details, or subscription tokens in public issues.

## License

Xingdu is licensed under the [MIT License](LICENSE). Personal and commercial use, modification, and redistribution are permitted under its terms. Retain the copyright and license notices when distributing copies or substantial portions of the software.

Third-party dependencies and protocol engines remain subject to their own licenses.
