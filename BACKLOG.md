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

## Organize mode

- **Multi-select / bulk actions** — select several saves and move, remove, or
  download them at once.
- **Drag & drop saves onto collections** in the left sidebar.
- **Grid virtualization** for very large libraries — the masonry currently
  keeps every loaded tile mounted; fine at hundreds of images, worth
  virtualizing in the thousands.

## Search

- **Search by color** (also listed on the public roadmap at
  /support-currents-project).
