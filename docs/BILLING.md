# Xingdu Cloud billing

The Cloud plan belongs to an **organization**, not an individual member:

| Interval | USD price | Included servers |
| --- | --- | --- |
| Free | $0 | 1 |
| Monthly | $5 each month | 10 |
| Yearly | $40 each year | 10 |

Yearly billing saves $20 versus twelve monthly payments (about 33%). Customers
provide their own VPS and bandwidth. Self-hosted deployments remain free under
MIT; existing operator-defined resource quotas still apply.

## Implementation and verification boundary

The repository implements Stripe-hosted Checkout, Customer Portal, signed
webhooks, PostgreSQL billing state, and Cloud resource creation limits. Local
provider simulation tests do not establish that a real Stripe account, payments,
webhook delivery, or production deployment works. Complete the test-mode flow
below before enabling live billing. Public launch also requires the operator's
actual service terms, privacy details, support contact and refund policy.

A local Stripe Sandbox check on 2026-09-29 verified the USD 40 yearly hosted
Checkout with a test card, a paid invoice, signed webhook delivery through the
Stripe CLI, and automatic activation in Xingdu. Customer Portal displayed the
invoice and canceled renewal at period end; the webhook preserved current paid
access. The running API rejected a duplicate subscription and a sixth server under
the previous five-server plan;
disposable host records were removed without connecting to any VPS. Both price
objects were validated against Stripe, but a monthly card payment and a public
production webhook were not part of this check. This is not live-payment or
production deployment acceptance.

## Configure a Stripe test account

Use a Stripe account dedicated to Xingdu. Store secrets in the API deployment's
environment or the ignored local `.env`; never put them in frontend variables,
Git, URLs, command arguments or chat messages.

1. Create a product called **Xingdu Cloud** with two recurring, flat-rate prices:
   **USD 5.00 every month** and **USD 40.00 every year**. Quantity is always one
   per organization. No trial, promotion code, adjustable quantity or usage-based
   charge is included in this initial integration.
2. Create a Billing Portal configuration with invoice history and payment-method
   updates enabled. Enable subscription cancellation **at the end of the billing
   period**. Disable subscription price/quantity updates. Set
   `STRIPE_PORTAL_CONFIGURATION` to its `bpc_...` ID. The API checks this policy
   before opening a portal session.
3. Set these backend variables (see `.env.example`):

   ```dotenv
   MODE=cloud
   STRIPE_SECRET_KEY=<test secret key>
   STRIPE_WEBHOOK_SECRET=<endpoint signing secret>
   STRIPE_PRICE_MONTHLY=<monthly price ID>
   STRIPE_PRICE_YEARLY=<yearly price ID>
   STRIPE_PORTAL_CONFIGURATION=<portal configuration ID>
   ```

   Cloud mode refuses to start with missing configuration. Each Checkout checks
   the selected price's currency, amount, recurring interval, licensed (non-metered) usage and Stripe mode.
   Omit `MODE`, or set `self_hosted`, to retain free self-hosting.
   API replicas must use the same deployment mode and Stripe configuration.
4. Register a **snapshot** webhook endpoint at
   `https://<your-origin>/api/v1/billing/stripe/webhook`. This implementation pins
   Stripe API version **2025-03-31.basil**; use that version for the endpoint.
   Subscribe to:

   - `checkout.session.completed`
   - `checkout.session.async_payment_succeeded`
   - `checkout.session.async_payment_failed`
   - `customer.subscription.created`
   - `customer.subscription.updated`
   - `customer.subscription.deleted`
   - `invoice.paid`
   - `invoice.payment_failed`

   The initial Checkout accepts cards. The async events are also handled for
   reconciliation. Stripe Connect accounts and mixed test/live events are rejected.
5. For local testing, run the official Stripe CLI with credentials held by the
   CLI, not in command arguments:

   ```sh
   stripe listen --events checkout.session.completed,checkout.session.async_payment_succeeded,checkout.session.async_payment_failed,customer.subscription.created,customer.subscription.updated,customer.subscription.deleted,invoice.paid,invoice.payment_failed --forward-to http://127.0.0.1:15173/api/v1/billing/stripe/webhook
   ```

   Put that listener's signing secret in the local environment, then restart the
   API. The CLI signing secret differs from the public endpoint's secret.
6. Open **Account menu → Plan & billing** as the organization owner. Complete a
   test Checkout, verify invoice payment and the persisted organization status,
   and confirm an eleventh server is rejected. Test both intervals, repeat clicks,
   cancel-at-period-end, payment failure, renewal and webhook retries. An admin
   or member must not be able to open Checkout or the billing portal.

Use new live-mode price IDs, portal configuration, signing secret and API secret
when preparing production. Configure tax collection and invoice requirements for
the operator's actual jurisdiction before launch; automatic tax is not enabled
by this implementation. Do not reuse test-mode objects for real payments.

## State and access policy

- Billing mutations recheck **owner** membership inside the organization lock.
  Members can read subscription status; Stripe IDs and Checkout URLs are not
  included in the status response. A returning Checkout URL preserves the
  organization selection but never grants access.
- Stripe customers are bound and committed locally before opening Checkout.
  Checkout creation uses a persisted idempotency seed and reuses an open session.
  Choosing a different interval expires the previous open session before creating
  another. Existing nonterminal subscriptions block a second Checkout.
- Webhooks verify the raw-body HMAC signature and a five-minute timestamp bound.
  Only the signed customer ID selects a billing record. Caller metadata cannot
  select an organization. Unknown customers are acknowledged without mutation.
- Webhooks take the same PostgreSQL organization lock as Checkout, fetch canonical
  Stripe state **after taking the lock**, and commit state plus event ID together.
  Duplicate events are ignored; out-of-order snapshots cannot restore old state.
  Provider or database failures return a retryable non-2xx response.
- The owner can use **Refresh billing status** to reconcile with Stripe if a
  webhook is delayed or missed. This uses server-to-server verification, never a
  client-provided status. Monitor failed webhook deliveries in Stripe; there is
  no separate scheduled reconciliation worker in this version.
- Paid access requires the configured price, one item with quantity one, an
  active subscription, a paid latest invoice, no collection pause and an unexpired
  period. Unexpected prices or item layouts do not grant access. Billing APIs
  fail closed when a customer has multiple nonterminal subscriptions or more than
  100 historical subscriptions; resolve these exceptional cases with the operator.
- In Cloud mode, each organization can manage one server for free, including
  protocol deployments and client subscriptions. An active paid plan allows ten
  servers. Above the current allowance, new hosts, deployments and client
  subscriptions are blocked; existing resources remain available. The eleventh
  server is rejected even
  for concurrent requests. Lower operator quotas still apply. Self-hosting does
  not use this payment gate. The API injects deployment mode into each scoped
  tenant transaction; tenants have no API for changing that mode or billing state.
- On expiry, existing machines, agents, deployed services and downloaded client
  configurations keep running. Inventory remains readable, and cleanup/removal
  remains available. Existing resource edits are retained; this version does not
  cut network access or erase configurations to enforce billing.
- Canceling through the portal stops renewal at period end; current paid access
  remains until then. Initial scope has **no active-plan interval switching or
  prorations**. Cancel renewal, then subscribe to the other interval after the
  current subscription ends. Refunds/disputes require operator handling; there is
  no automated refund entitlement workflow.

Billing tables use forced RLS under the non-owner runtime role. Account login,
membership, organization ownership and billing are separate concepts; ownership
transfer makes the new owner the billing manager without changing Stripe's
customer association. Treat the ownership-transfer permission accordingly.

## Local checks

`make check` includes provider transport, signature, status, authorization and
frontend checks. Set `XINGDU_TEST_DATABASE_URL` to a dedicated PostgreSQL test
database to additionally exercise owner/member isolation, webhook idempotency,
retry rollback, paid/unpaid transitions and concurrent ten-server enforcement.
The test suite uses an HTTP provider simulator; it does not charge a card.

Stripe references: [Checkout](https://docs.stripe.com/api/checkout/sessions/create),
[webhooks](https://docs.stripe.com/webhooks),
[portal configuration](https://docs.stripe.com/api/customer_portal/configurations/create).
