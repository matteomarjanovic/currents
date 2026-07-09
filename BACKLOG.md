# Backlog — deferred features

Things we know we want but have deliberately postponed. Check this file before
proposing new work; delete an entry when it ships.

## Supporter launch operations

- **Embedding backfill as a deploy routine.** Saves indexed while the inference
  server was unreachable have `visual_identity_id IS NULL` and are invisible to
  the paid library search and find-similar. Run the repair pass
  (`APPVIEW_MODE=repair`) after deploys/outages so paid search coverage stays
  complete — and consider scheduling it.
- **Polar webhook-miss backfill.** If webhooks are missed for longer than the
  retry window, re-sync `GET /v1/subscriptions` from the Polar API into
  `polar_subscription` (see POLAR.md "Not built yet").
- **Colors backfill as a deploy step.** Visual identities enriched before the
  color-search deploy have no `visual_identity_color` rows and are invisible to
  search-by-color until `appview backfill-colors` runs (one-shot, resumable).

## Organize mode

- **Multi-select / bulk actions** — select several saves and move, remove, or
  download them at once.
- **Drag & drop saves onto collections** in the left sidebar.
- **Grid virtualization** for very large libraries — the masonry currently
  keeps every loaded tile mounted; fine at hundreds of images, worth
  virtualizing in the thousands.

## Engineering

- **CI for the test suites.** GitHub Actions workflow running on PRs: appview
  `go test ./...` twice — plain, and with `TEST_DATABASE_URL` against a
  `pgvector/pgvector` service container (the suite creates and migrates its own
  `_test` database); frontend `npm run test:unit` + lint; inference
  `python -m unittest` (needs the Python deps but no model download); clustering
  `python -m unittest` via its Docker image. Playwright e2e optional/later
  (heaviest, needs browsers).
