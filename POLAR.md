# Polar — Supporter tier

Semantic library search (`is.currents.feed.searchLibrarySaves`) and find-similar-in-library
(`is.currents.feed.findSimilarInLibrary`) are gated behind a supporter subscription
($7/month or $70/year), sold through Polar (merchant of record).

## How it works

- **The gate is one env var.** With `POLAR_WEBHOOK_SECRET` unset on the appview, every
  authenticated user counts as a supporter (dev environments need no Polar account, and
  deploying the code before configuring Polar changes nothing). Setting it turns the
  paywall on.
- **Entitlement is a webhook mirror.** Polar POSTs subscription lifecycle events to
  `POST /api/polar/webhook` (`appview/supporter.go`). After Standard-Webhooks signature
  verification, every `subscription.*` event is upserted into the `polar_subscription`
  table, keyed by the Polar subscription id. A user is a supporter while any of their
  rows has status `active`, `trialing`, or `past_due` (grace while Polar retries
  payment). A mid-period cancel keeps status `active` (with `ends_at` set) until it
  takes effect — no special handling needed; `subscription.revoked` then flips it.
- **The user ↔ subscription mapping is the external customer id.** The appview creates
  every checkout session server-side (`POST /api/supporter/checkout`) and stamps the
  viewer's DID as Polar's `external_customer_id`; Polar echoes it back as
  `data.customer.external_id` on every subscription event.
- **Enforcement is server-side**: the two XRPC handlers return `403 SupporterRequired`
  for non-supporters. The client mirrors the status via `GET /api/supporter/status`
  (`frontend/src/lib/stores/supporter.svelte.ts`, loaded by the organize layout) and
  shows the upgrade dialog (`frontend/src/lib/components/supporter-dialog.svelte`) when a
  non-supporter clicks "Find similar in library" or starts typing in the search command.
- **Checkout is embedded.** `openSupporterCheckout` (`frontend/src/lib/polar.ts`) asks
  the appview for a checkout session URL (created with `embed_origin` = `FRONTEND_URL`)
  and opens it with `@polar-sh/checkout`'s `PolarEmbedCheckout` overlay — no client
  token or Polar config exists in the frontend beyond the two public product ids.
- **Provisioning is webhook-driven, not redirect-driven.** After the embed fires its
  `success` event, the client polls the status endpoint a few times until the mirror
  catches up (usually the first attempt).
- **Subscription management is Polar's customer portal.** The settings dialog's
  Subscription section (`frontend/src/lib/components/settings-dialog.svelte`) shows the
  plans to non-subscribers and a "Manage" button to subscribers; the latter calls
  `POST /api/supporter/portal`, which mints a customer session via the Polar API
  (`POLAR_ACCESS_TOKEN`) and returns the portal URL — invoices, payment method, plan
  changes, and cancellation all happen there.

## Setup (sandbox first, then live)

Sandbox ([sandbox.polar.sh](https://sandbox.polar.sh)) and production
([polar.sh](https://polar.sh)) are separate Polar environments with separate
organizations, tokens, product ids, and webhook secrets. Do everything below in
sandbox first; repeat in production when ready. `POLAR_SERVER` on the appview picks
the environment (`sandbox` / `production`, default `production`) — the token itself
doesn't encode it.

1. **Catalog** — Polar > Products: create two products, `Currents Supporter (monthly)`
   at **$7 / month** and `Currents Supporter (yearly)` at **$70 / year** (Polar
   products have one price each). Note the two product UUIDs.
2. **Access token** — Polar > Settings > Developers: create an organization access
   token (`polar_oat_...`). Server-side secret. Scopes: `checkouts:write` and
   `customer_sessions:write`; optionally `subscriptions:read` for the future
   webhook-miss backfill.
3. **Webhook endpoint** — Polar > Settings > Webhooks > Add endpoint:
   - URL: `https://api-dev.currents.is/api/polar/webhook` (sandbox) /
     `https://api.currents.is/api/polar/webhook` (live)
   - Format: **Raw**.
   - Events: tick all `subscription.*` events (created, updated, active, canceled,
     uncanceled, revoked). The handler converges them all with one upsert.
   - Generate a secret and note it — it's also the appview's paywall switch.
4. **Env vars**:
   - Appview: `POLAR_WEBHOOK_SECRET=...` (this is also the paywall switch),
     `POLAR_ACCESS_TOKEN=polar_oat_...` (enables checkout + billing portal), and
     `POLAR_SERVER=sandbox` while testing against sandbox.
   - Frontend (committed, not secrets): sandbox product UUIDs in
     `frontend/.env.development` (what `npm run dev` uses), production UUIDs in
     `frontend/.env.production` (what production builds — Netlify — use; env vars set
     in the Netlify dashboard override the file, and are baked at build time, so any
     change there needs a redeploy): `PUBLIC_POLAR_PRODUCT_MONTHLY`,
     `PUBLIC_POLAR_PRODUCT_YEARLY`. While the ids are empty the subscribe buttons stay
     disabled. Never point production at sandbox products — visitors would see working
     test checkouts.

## Testing in sandbox

- Complete a checkout with the Stripe test card `4242 4242 4242 4242`, any future
  expiry, any CVC. The `subscription.created` webhook should land within a second —
  check the appview log for `polar subscription synced` and the row in
  `polar_subscription`.
- Failed deliveries are retried with exponential backoff (up to 10 attempts), and are
  replayable from the endpoint's delivery log in the dashboard. The handler answers
  non-2xx on any failure precisely so Polar retries. After 10 straight failures Polar
  disables the endpoint — re-enable it from the dashboard after fixing.

## Not built yet

- **Backfill** — if webhooks are missed for longer than the retry window, re-sync from
  the Polar API (`GET /v1/subscriptions`) into `polar_subscription`.
