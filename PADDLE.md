# Paddle — Supporter tier

Semantic library search (`is.currents.feed.searchLibrarySaves`) and find-similar-in-library
(`is.currents.feed.findSimilarInLibrary`) are gated behind a supporter subscription
($7/month or $70/year), sold through Paddle.

## How it works

- **The gate is one env var.** With `PADDLE_WEBHOOK_SECRET` unset on the appview, every
  authenticated user counts as a supporter (dev environments need no Paddle account, and
  deploying the code before configuring Paddle changes nothing). Setting it turns the
  paywall on.
- **Entitlement is a webhook mirror.** Paddle POSTs subscription lifecycle events to
  `POST /api/paddle/webhook` (`appview/supporter.go`). After HMAC signature verification,
  every `subscription.*` event is upserted into the `paddle_subscription` table, keyed by
  the Paddle subscription id. A user is a supporter while any of their rows has status
  `active`, `trialing`, or `past_due` (grace while Paddle retries payment). A mid-period
  cancel keeps status `active` (with `scheduled_change` set) until it takes effect — no
  special handling needed.
- **The user ↔ subscription mapping is `custom_data.did`.** The frontend passes the
  viewer's DID as checkout custom data (`frontend/src/lib/paddle.ts`); Paddle echoes it
  back on every subscription event.
- **Enforcement is server-side**: the two XRPC handlers return `403 SupporterRequired`
  for non-supporters. The client mirrors the status via `GET /api/supporter/status`
  (`frontend/src/lib/stores/supporter.svelte.ts`, loaded by the organize layout) and
  shows the upgrade dialog (`frontend/src/lib/components/supporter-dialog.svelte`) when a
  non-supporter clicks "Find similar in library" or starts typing in the search command.
- **Provisioning is webhook-driven, not redirect-driven.** After `checkout.completed`,
  the client polls the status endpoint a few times until the mirror catches up (usually
  the first attempt).
- **Subscription management is Paddle's customer portal.** The settings dialog's
  Subscription section (`frontend/src/lib/components/settings-dialog.svelte`) shows the
  plans to non-subscribers and a "Manage" button to subscribers; the latter calls
  `POST /api/supporter/portal`, which mints a portal session via the Paddle REST API
  (`PADDLE_API_KEY`; sandbox vs live inferred from the key's `pdl_sdbx_` prefix) and
  returns the overview URL — invoices, payment method, plan changes, and cancellation
  all happen there.

## Setup (sandbox first, then live)

Sandbox and live are separate Paddle environments with separate tokens, price ids, and
webhook secrets. Do everything below in the [sandbox dashboard](https://sandbox-vendors.paddle.com/)
first; repeat in [live](https://vendors.paddle.com/) when ready.

1. **Catalog** — Paddle > Catalog > Products: create product `Currents Supporter`
   (tax category: Standard digital goods) with two recurring prices: **$7 / month** and
   **$70 / year**. Note the two price ids (`pri_...`).
2. **Client token** — Paddle > Developer tools > Authentication > Client tokens: create
   one (`test_...` in sandbox, `live_...` in live).
3. **Webhook destination** — Paddle > Developer tools > Notifications > New destination:
   - URL: `https://api-dev.currents.is/api/paddle/webhook` (sandbox) /
     `https://api.currents.is/api/paddle/webhook` (live)
   - Usage type: tick both **Platform** (real events) and **Simulation** (events from
     the webhook simulator).
   - Events: tick all `subscription.*` events (created, updated, canceled, activated,
     paused, resumed, past_due, trialing). The handler converges them all with one upsert.
   - The destination's **secret key** (`pdl_ntfset_...`) is viewable anytime via the
     destination's **⋯ menu > Edit destination**. It is not an API key — don't confuse
     the two.
4. **API key** — Paddle > Developer tools > Authentication > API keys: create one
   (`pdl_sdbx_apikey_...` / `pdl_live_apikey_...`). Server-side secret. Minimal
   permissions: **Customer portal sessions → Write** (`customer_portal_session.write`)
   is all the appview uses; optionally **Subscriptions → Read** for the future
   webhook-miss backfill. Permissions can be edited on the key later.
5. **Checkout settings** — Paddle > Checkout > Checkout settings: set a default payment
   link (`https://currents.is/` is fine) and make sure the website domain is approved
   (automatic in sandbox). Skipping the payment link makes every checkout fail with the
   generic "Something went wrong" dialog — the underlying error is a 400 from
   `transaction-checkout` with `transaction_default_checkout_url_not_set`.
6. **Env vars**:
   - Appview: `PADDLE_WEBHOOK_SECRET=pdl_ntfset_...` (this is also the paywall switch)
     and `PADDLE_API_KEY=pdl_..._apikey_...` (enables the billing portal; sandbox vs
     live is inferred from the key prefix).
   - Frontend (committed, none of these are secrets): sandbox values in
     `frontend/.env.development` (what `npm run dev` uses — it overrides `.env`), live
     values in `frontend/.env` (what production builds use):
     `PUBLIC_PADDLE_ENVIRONMENT`, `PUBLIC_PADDLE_CLIENT_TOKEN`,
     `PUBLIC_PADDLE_PRICE_MONTHLY`, `PUBLIC_PADDLE_PRICE_YEARLY`. The price vars take
     `pri_...` ids from step 1, not dollar amounts. While the token or price ids are
     empty the subscribe buttons stay disabled ("Payments aren't configured in this
     environment yet").

## Testing in sandbox

- Complete a checkout with the test card `4242 4242 4242 4242`, any future expiry, any
  CVC. The `subscription.created` webhook should land within a second — check the
  appview log for `paddle subscription synced` and the row in `paddle_subscription`.
- Paddle > Developer tools > Simulations can fire signed `subscription.*` events at the
  destination without a checkout (useful for `paused` / `canceled` transitions).
- Failed deliveries are retried (sandbox: 3 attempts over ~15 min; live: 60 attempts
  over ~3 days) and are replayable from Developer tools > Notifications > destination
  logs. The handler answers non-2xx on any failure precisely so Paddle retries.

## Not built yet

- **Backfill** — if webhooks are missed for longer than the retry window, re-sync from
  the Paddle API (`GET /subscriptions`) into `paddle_subscription`.
