# Account security

Account profile and security pages are functional for authenticated users, independently of the selected organization. Display-name edits are stored under PostgreSQL row-level security. The immutable login username and organization role remain distinct.

Password changes require the current password and a new password of 12–72 bytes. The endpoint requires session authentication and CSRF protection, limits attempts, and bounds simultaneous bcrypt work. Password verification/hashing happens before a database transaction. The database atomically compares the previous hash and a live current session, changes the hash, and revokes **all** sessions, including the caller. A concurrent login must recheck its verified password snapshot under the same account lock before issuing a session; a stale password cannot produce a session after a completed change. The runtime role has no general UPDATE grant on users; a narrowly scoped function performs this transition.

The session list exposes random UUID identifiers, creation and expiry times, and a current-session flag. It never exposes bearer tokens or token hashes. Revocation is scoped to the authenticated user; organization administrators cannot enumerate or revoke other users' sessions. Session expiry remains 24 hours. These are login sessions, not an asserted physical-device inventory: device fingerprints, IP locations and user-agent tracking are not collected.

The current implementation does **not** provide password recovery, TOTP/MFA or organization deletion. These should not be advertised as available. Password changes and session revocation do not revoke independent Agent credentials or public subscription tokens.

Validation: PostgreSQL integration tests run under `xingdu_app` and cover cross-account session revocation, profile RLS, stale password/current-session rejection, session invalidation and stale verified login rejection. HTTP integration verifies CSRF, wrong-password handling, redacted session responses and forced reauthentication after a successful change.

Organization owners can transfer ownership to an existing member after confirming their username and supplying the current password. The former owner becomes an administrator. A transaction serializes membership changes, rechecks password/session and ownership, changes both roles atomically, and writes an organization audit event. A dedicated non-login function role remains subject to organization-scoped RLS; ordinary member APIs still cannot mutate owner rows. Integration tests cover invalid/cross-organization targets, unauthorized callers, stale passwords, and competing transfers preserving exactly one owner.

Public registration now requires email verification through Resend; see [email registration](EMAIL-REGISTRATION.md). Missing provider configuration fails closed.
