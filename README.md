# Xingdu · 星渡

**Your servers. Your routes. One place to manage them.**

[English](README.md) · [简体中文](README.zh-CN.md)

[![CI](https://github.com/Xingdu-App/xingdu/actions/workflows/ci.yml/badge.svg)](https://github.com/Xingdu-App/xingdu/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Xingdu is an open-source project building a self-hosted control panel for VPS and proxy infrastructure. Its goal is to bring server onboarding, protocol deployment, direct and relay routes, and client subscriptions into one workflow.

Built for individuals and small teams, Xingdu is designed to work with different protocol engines and clients, without tying your infrastructure to a single client application.

> **Early development preview.** You can run the console locally today. Authentication, VPS onboarding, protocol deployment, and subscription export are not yet available. Keep this preview local; it is not ready for public or production deployment.

## What you can try today

- **Web console:** responsive navigation, an empty server inventory, and clear placeholders for upcoming features.
- **Live service status:** API and database availability, with error messages and retry when the connection fails.
- **Local Docker setup:** starts the web console, API, PostgreSQL, database migrations, and a worker process.
- **Development foundation:** Go and TypeScript code, automated checks, and separate worker and agent entry points.

The worker does not execute deployment jobs yet, and the agent does not enroll or configure servers. The current console UI is in Simplified Chinese.

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

Open the console at **[http://127.0.0.1:15173](http://127.0.0.1:15173)**. The first build downloads dependencies and container images, so it may take a few minutes.

A fresh installation shows an empty server list. Adding a server is intentionally disabled until onboarding is implemented.

| Service | Local address |
| --- | --- |
| Web console | `http://127.0.0.1:15173` |
| API | `http://127.0.0.1:18080` |
| PostgreSQL | `127.0.0.1:54329` |

All published ports bind to loopback. The example credentials are for local development only. If you already have a `.env` file, keep it instead of copying over it.

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
| [Development guide](docs/DEVELOPMENT.md) | Local setup, commands, and API endpoints |
| [Development plan](docs/PLAN.md) | Scope and milestones |
| [Adapter design](docs/ADAPTERS.md) | Runtime and client integration boundaries |
| [Contributing](CONTRIBUTING.md) | Contribution guidelines |
| [Security](SECURITY.md) | Vulnerability reporting guidance |

Supporting documents are currently in Simplified Chinese. Do not include credentials, private server details, or subscription tokens in public issues.

## License

Xingdu is licensed under the [MIT License](LICENSE). Personal and commercial use, modification, and redistribution are permitted under its terms. Retain the copyright and license notices when distributing copies or substantial portions of the software.

Third-party dependencies and protocol engines remain subject to their own licenses.
