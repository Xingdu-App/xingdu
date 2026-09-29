# Verified email registration

Public registration uses an email address, password and organization name. It creates a pending challenge only. The user enters the eight-digit emailed code before an account, owner membership or organization is created. New accounts use the normalized email as the login username and store `email_verified_at`; existing username-only administrators remain valid. Existing accounts cannot be overwritten or have their passwords reset through registration.

API:

- `POST /api/v1/auth/register` with `email`, `password`, `organization` returns HTTP 202 with `registration_token` and `expires_in: 600`. This is not registration success.
- `POST /api/v1/auth/register/verify` with `registration_token`, `code` returns HTTP 201 only after the account transaction commits. Login then uses the email in the existing `username` field.
- Resending repeats the first call. There is a persistent 60-second email cooldown and at most five requests per email in a rolling 24 hours. A new challenge invalidates previous ones.

Challenge tokens have 256 bits of randomness and only their hashes are stored. Codes are hashed together with the random token, so a database dump does not permit enumerating the eight-digit code without the unstored token. Challenges expire after ten minutes, allow at most five valid-format guesses, and are consumed exactly once under a row lock. Accounts are created atomically with verification. IP/global request limits and bounded bcrypt work provide additional process-local protection; proxy-aware/distributed IP limiting remains an operational concern.

`RESEND_API_KEY` and `XINGDU_EMAIL_FROM` configure the adapter. For example, the sender can be `Xingdu <noreply@xingdu.app>` after the domain is verified with Resend. Blank, placeholder/example keys are treated as unconfigured. `email_delivery_configured` describes local configuration validity, not proven provider readiness. Missing configuration or a failed provider request returns `email_unavailable`, leaves the challenge inactive, and never creates or auto-verifies an account. Disabling registration also disables verification.

The adapter follows the [Resend send-email API](https://resend.com/docs/api-reference/emails/send-email), uses a fixed HTTPS endpoint, a six-second timeout and an idempotency key. Redirects are rejected; provider error bodies, codes and credentials are never logged or returned. No real email was sent during implementation: tests use an in-memory sender and mock HTTP transport.

To avoid registration enumeration, syntactically valid addresses receive the same challenge flow and generic email regardless of account existence. A challenge for an existing account cannot recreate it; verification returns the same invalid-verification error. Requests remain rate limited. Pending authentication data is retained for at most a rolling day during registration traffic; a later scheduled retention job can enforce cleanup during idle periods.

Password recovery and account notifications are described in [transactional email](TRANSACTIONAL-EMAIL.md). Email-address changes, MFA and migrating username-only administrators to verified email remain separate features. Actual Resend domain verification, key provisioning and delivery acceptance must happen before claiming production email delivery works.

## Proxy rate-limit boundary

Registration currently uses the direct TCP peer from `RemoteAddr`, never an untrusted `X-Forwarded-For` header. Behind the preview Nginx proxy this deliberately groups visitors into one bucket: five registration requests per minute per API process. Email-specific cooldowns and daily limits remain database-backed across processes. This restrictive preview limit can reject unrelated concurrent users. Before public scaling, configure and test an explicit trusted-proxy CIDR/forwarding boundary or enforce visitor IP limits at a trusted ingress; do not blindly accept client-supplied forwarded headers.
