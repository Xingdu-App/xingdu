# Transactional email

The API uses Resend with `RESEND_API_KEY` and `XINGDU_EMAIL_FROM` (default:
`Xingdu <noreply@xingdu.app>`). Verify the sender domain in Resend before enabling
real delivery. `XINGDU_PUBLIC_ORIGIN` supplies console/invitation links. Support
replies go to `info@xingdu.app`. Keep the provider key in the API service only.
Configuration checks do not establish inbox delivery.

Templates include bilingual Chinese/English HTML and plain text, inline email
styles, escaped user-provided text, and safe action URLs. Authentication codes
remain text, with a ten-minute expiry. Preview all templates offline with
`go run ./tools/preview-emails`; output defaults to ignored `.local/email-previews`.
No email is sent by the preview command.

| Event | Trigger and delivery |
| --- | --- |
| Registration code | Existing registration request; synchronous send, challenge activated only after provider acceptance |
| Welcome | New verified-email user transaction, including OAuth-created accounts |
| Password recovery code | Login page recovery request; synchronous send, same response/email flow for unknown and existing addresses |
| Password changed | Committed password change or recovery, after invalidating old sessions |
| Organization invitation | Optional email on invitation creation; provider acceptance/failure returned separately from successful link creation |
| Subscription active | Billing reconciliation transitions to active/trialing |
| Billing attention | State changes to past_due, unpaid or incomplete |
| Cancellation scheduled | cancel_at_period_end changes; payment problems take priority |
| Subscription ended | canceled or incomplete_expired |
| Subscription updated | Other plan/status/cancellation transitions |

Billing notices are state-change notifications to the current owner's verified
email, not invoice receipts. Renewal without a status/plan change does not emit
a notice. Duplicate webhook processing and unchanged reconciliation do not
produce duplicate notices. Accounts without a verified email are skipped.

## Durable notification queue

Migration 037 adds a transactional outbox. Account and billing triggers enqueue
only after a real database change, in the same transaction. API replicas claim
one record with SKIP LOCKED and a one-minute lease; completion checks the lease
token. Provider calls use a stable idempotency key, JSON HTML/plain text, six-second
timeout, and no redirects. Retries use backoff, at most ten attempts and a
20-hour retry window (within Resend's 24-hour idempotency retention). Failed
records stay marked for operator review; queue records expire after 30 days.
Cleanup runs when delivery is configured and the dispatcher polls.

The `xingdu_mail` role is NOLOGIN and NOBYPASSRLS. It owns narrowly scoped
trigger, claim, completion and password-recovery functions, not tables. Runtime
roles cannot read/write the delivery queue directly or assume this role. Queue
RLS grants service access only to that role. Billing recipient lookup uses
transaction-local `app.mail_org` scope with dedicated RLS policies. The API
process runs the dispatcher; the machine Worker never receives the mail key.
Provider responses, codes, addresses and credentials are not logged.

Invitation links retain their existing bearer semantics: one use, seven days,
not bound to the recipient mailbox. Sending to an email does not grant access
until the link is accepted by a logged-in user. Failed invitation delivery keeps
the created link available for manual sharing; no plaintext invitation tokens
are persisted in the mail queue. Delivery acceptance is not inbox confirmation.

## Password recovery

- `POST /api/v1/auth/password-recovery` with `email` returns a random
  `recovery_token` and `expires_in: 600`; keep the token in page memory.
- `POST /api/v1/auth/password-recovery/complete` with `recovery_token`, `code`
  and `new_password` returns 204 on success.
- Same-origin request checks apply. Code/token hashes are stored, with persistent
  60-second cooldown, five sends per address per day, five code attempts, plus
  process IP/global limits and bounded bcrypt work.
- Only verified accounts with password login enabled can recover. Social-only
  accounts continue through their provider; username-only accounts need admin
  support. Recovery cannot enable a disabled password method.
- Password snapshots reject challenges issued before a password change.
  Completion atomically consumes challenges, changes the password and revokes
  all login sessions under the same account lock used by password changes/login.
  Registration availability does not control recovery availability.

Production acceptance requires actual provider configuration, domain verification
and an authorized recipient test. Automated tests use fake mail transports and
a dedicated PostgreSQL database, never real recipient addresses.
