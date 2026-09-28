# Contributing to Xingdu

Keep client exporters independent of server runtimes. Preserve organization isolation and use the restricted PostgreSQL runtime role for business operations. Never add credentials, real subscriptions, private deployment data or personal filesystem paths to the repository.

Run `make check` before submitting code. Changes involving data or authorization need the dedicated PostgreSQL integration suite; UI changes also need a browser check. See [development](docs/DEVELOPMENT.md) and [implementation status](docs/IMPLEMENTATION-STATUS.md).

## Commit messages

Use an English imperative subject such as `fix(auth): require email verification before creating accounts`. A behavior change should explain why the old behavior was a problem, what now happens, and the key choices and limits a reviewer needs to understand. Record the validation actually performed in a separate paragraph. Wrap body lines around 72–80 characters.

For example (illustrative, not a claim that these checks ran):

```text
fix(auth): reject unverified registration challenges

A pending registration could previously be treated as an active account.
Require the challenge to be verified and consumed before creating the user
and their first organization in one transaction.

Keep existing administrator logins available. Missing email-provider
configuration disables new registration instead of bypassing verification.

Validation: describe the actual tests and environment used here. Identify
any live email or production verification that remains pending.
```

Keep independently reviewable requirements in separate commits when practical. Stage only relevant files and inspect the staged diff. Include real issue links when applicable. Do not rewrite published history as part of message cleanup.

Contributors should use their own appropriate public identity. Automation for this repository uses `Xingdu <noreply@xingdu.app>` and credits actual Codex work with `Co-Authored-By: Codex <noreply@openai.com>`. Do not impersonate other contributors or reuse identities from historical examples.

An optional commented template is available in `.gitmessage`:

```sh
git config --local commit.template .gitmessage
```

A successful commit or push is separate from a successful deployment. Release notes should distinguish source changes, local tests, actual client/runtime acceptance and public availability.
