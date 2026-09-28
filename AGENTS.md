# Xingdu repository guidance

- Keep this project independent of Stash; runtime and client adapters have separate boundaries.
- Agent-authored commits in this checkout must use `Xingdu <noreply@xingdu.app>` for both author and committer. Verify effective identity before committing. Never change global Git identity.
- Never add personal email addresses, local absolute paths, secrets, live VPS addresses, or real subscriptions to tracked files.
- Do not claim a runtime/client combination works until verified. Distinguish scaffold, implemented, and end-to-end verified capabilities.
- Use `make check` for backend and frontend checks. Use `docker compose up --build -d --wait` for integration validation when Docker is available.
- The current scaffold is local development only. Multi-user authentication, organization membership/invitations, PostgreSQL RLS and host inventory are implemented; Agent enrollment, telemetry and SSH bootstrap are implemented; managed Agent protocol installation/uninstallation is implemented for five TLS protocols; route orchestration and subscription export are not implemented; keep published development ports on loopback.

- Tenant data must use transaction-local organization/user scope and a non-owner runtime database role. Never bypass RLS for API or worker business operations. Follow docs/SAAS.md for role boundaries.

- Machine credentials must never be logged, returned or passed through command arguments. Require pinned SSH host keys, reject unsafe network targets, and keep agent machine auth separate from browser sessions. Read docs/MACHINE-ACCESS.md before changing machine access.
