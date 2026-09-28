# Xingdu repository guidance

- Keep this project independent of Stash; runtime and client adapters have separate boundaries.
- Agent-authored commits in this checkout must use `Xingdu <noreply@xingdu.app>` for both author and committer. Verify effective identity before committing. Never change global Git identity.
- Never add personal email addresses, local absolute paths, secrets, live VPS addresses, or real subscriptions to tracked files.
- Do not claim a runtime/client combination works until verified. Distinguish scaffold, implemented, and end-to-end verified capabilities.
- Use `make check` for backend and frontend checks. Use `docker compose up --build -d --wait` for integration validation when Docker is available.
- The current scaffold is local development only. Single-admin authentication and host inventory are implemented; Agent enrollment and deployment execution are not implemented; keep published development ports on loopback.
