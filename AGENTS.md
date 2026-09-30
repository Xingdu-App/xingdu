# Xingdu repository guidance

- Keep this project independent of Stash; runtime and client adapters have separate boundaries.
- Agent-authored commits in this checkout must use `Xingdu <noreply@xingdu.app>` for both author and committer. Verify effective identity before committing. Never change global Git identity.
- Never add personal email addresses, local absolute paths, secrets, live VPS addresses, or real subscriptions to tracked files.
- Do not claim a runtime/client combination works until verified. Distinguish scaffold, implemented, and end-to-end verified capabilities.
- Use `make check` for backend and frontend checks. Use `docker compose up --build -d --wait` for integration validation when Docker is available.
- Local validation does not establish production readiness. Consult docs/IMPLEMENTATION-STATUS.md for current capabilities, released-artifact boundaries and recorded acceptance evidence; docs/PROTOCOL-DEPLOYMENT.md and docs/SUBSCRIPTIONS.md define protocol/client restrictions. Configuration revisions and recovery, single TCP relays, operator-run probes and ACME, and Agent release CI are implemented; deployment and real client acceptance remain specific to tested combinations. Keep published development ports on loopback.

- Tenant data must use transaction-local organization/user scope and a non-owner runtime database role. Never bypass RLS for API or worker business operations. Follow docs/SAAS.md for role boundaries.

- Machine credentials must never be logged, returned or passed through command arguments. Require pinned SSH host keys, reject unsafe network targets, and keep agent machine auth separate from browser sessions. Read docs/MACHINE-ACCESS.md before changing machine access.

## Commits and review

- Use an English imperative subject in `type: summary` or `type(scope): summary` form (`feat`, `fix`, `refactor`, `docs`, `test`, `build`, `ci`, `chore`). Keep each independently reviewable requirement in its own commit when possible.
- Behavior changes need a substantive body: explain the previous behavior and concrete problem, then the resulting behavior and implementation choices that matter. Include material limitations or tradeoffs.
- Record only checks actually completed and what they establish. Distinguish unit/integration tests, builds, local containers, real client/device checks and deployed public endpoints. A push is not deployment verification.
- Write connected paragraphs separated by blank lines; wrap body lines around 72–80 characters. Do not substitute a file inventory or repeat the subject for an explanation.
- Preserve the `Xingdu <noreply@xingdu.app>` author and committer identity. Credit actual AI contributions with `Co-Authored-By: Codex <noreply@openai.com>`; do not copy another assistant identity from examples.
- Link a relevant issue before the collaborator trailer when one exists; never invent one. Do not rewrite published history without explicit authorization.
- Inspect the staged diff, stage intended paths/hunks only, and exclude secrets, personal paths, local lab state and generated build output. Verify effective identity before commit and remote synchronization after a requested push.
- Ordinary edits do not imply permission to publish. An explicit deployment request authorizes the source commit/push required for that deployment; otherwise follow the user's requested commit/push scope.
