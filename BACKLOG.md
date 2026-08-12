# Backlog — deferred features

Things we know we want but have deliberately postponed. Check this file before
proposing new work; delete an entry when it ships.

## Supporter launch operations

- **Embedding backfill as a deploy routine.** Saves indexed while the inference
  server was unreachable have `visual_identity_id IS NULL` and are invisible to
  the paid library search and find-similar. Run the repair pass
  (`APPVIEW_MODE=repair`) after deploys/outages so paid search coverage stays
  complete — and consider scheduling it.
- **Colors backfill as a deploy step.** Visual identities enriched before the
  color-search deploy have no `visual_identity_color` rows and are invisible to
  search-by-color until `appview backfill-colors` runs (one-shot, resumable).

## Organize mode

- **Drag & drop saves onto collections** in the left sidebar.
- **Grid virtualization** for very large libraries — the masonry currently
  keeps every loaded tile mounted; fine at hundreds of images, worth
  virtualizing in the thousands.

## Engineering

- **Move the appview stack to a Scaleway Virtual Instance.** Motivation: the
  2026-08-04 incident — the mac mini's ISP had a degraded route to
  `relay1.us-west` and TAP fell hours behind (mitigated by pinning
  `relay1.us-east`); a datacenter VM is immune to residential peering
  roulette and moves the full-firehose bandwidth off the home line. The
  plan, compose file, and cost/growth analysis are written up:
  `SCALEWAY_MIGRATION.md` (step-by-step, `docker-compose.scaleway.yml`) and
  `SCALING.md` (phased plan for when the vector data grows). Only inference
  stays on the mac mini (reached over Tailscale); clustering runs on the VM
  and publishes the monthly UMAP model through an Object Storage model store
  that the mini syncs on a cron.

- **Collapse the per-field save-edit endpoints into one.** Editing a save's
  metadata is currently spread over four handlers — `PUT /save/{id}/alt`,
  `PUT /save/{id}/labels`, `PUT /save/attribution`, `PUT /save/labels/bulk` —
  each re-implementing "read the record, change one field, write the whole thing
  back" (`RepoPutRecord` replaces the record, so every untouched field has to be
  copied across or it's lost). A single PATCH-style endpoint taking optional
  fields would be the better design; `/alt` followed the existing per-field
  pattern rather than breaking it mid-feature. Two notes for whoever does it:
  `putSaveContentForRkey` already unifies the alt and attribution rewrites, so
  the content path is half-done; labels genuinely differ (add-only merge, and
  resaves are refused) and shouldn't be flattened into the same semantics.
  Related: **`PUT /save/{id}` looks like dead code** — it 302s to `/save`, a
  server-rendered page that no longer exists, and no client calls it. Worth
  deleting in the same pass, after one more grep for external consumers.
- **CI for the test suites.** GitHub Actions workflow running on PRs: appview
  `go test ./...` twice — plain, and with `TEST_DATABASE_URL` against a
  `pgvector/pgvector` service container (the suite creates and migrates its own
  `_test` database); frontend `npm run test:unit` + lint; inference
  `python -m unittest` (needs the Python deps but no model download); clustering
  `python -m unittest` via its Docker image. Playwright e2e optional/later
  (heaviest, needs browsers).
