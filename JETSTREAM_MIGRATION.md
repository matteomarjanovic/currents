# Replacing TAP with Jetstream v2

Jetstream can send only `is.currents.*` commits to the appview. The staging
shadow probe in `appview/cmd/jetstream-shadow` measures delivery and compares
the commits with TAP's repository mirror. It does not write appview data.

On 2026-10-03, a ten-minute Mac mini staging run received 547 filtered
events (335,207 JSON bytes): 260 commits, 129 identity, 156 account, and 2
sync markers. All 260 commits matched TAP's repository mirror after a
15-second settling window. This sample shows the filtered transport is small
and that TAP was current during this run; it does not establish behavior after
a long outage or across a cursor gap.

## Behavior the replacement must preserve

- Process collection, save, follow, favourite, and profile records through the
  existing `handleTapRecord` path. Keep URI-keyed writes idempotent.
- Handle identity events so cached actor handles follow DID changes. Handle
  account deletions and sync divergence markers by removing a repository's
  indexed records before replacement commits are applied.
- Keep a durable Jetstream sequence cursor. Advance it only after each event's
  database work succeeds. Replay from that cursor after a restart; never start
  from the current tip when a saved cursor exists. A redelivered event must be
  safe to apply again.
- Preserve Currents account deletion. A deleted account whose PDS records were
  retained must stay excluded from indexing until that DID logs in again. A
  durable opt-out row is required because Jetstream delivers every network
  repository matching the collection filter. Login clears the opt-out and
  queues a PDS backfill.
- Backfill existing records when a DID first publishes a Currents profile or
  returns after account deletion. The job must survive restarts and use the
  PDS as authority; a live event stream alone cannot restore old records.
- Reconcile after an unavailable PDS, cursor gap, or sync marker. Never treat a
  failed or partial PDS enumeration as an empty repository. Serialize the scan
  with event handling for that DID so a newer commit cannot be removed by a
  stale scan. See `REPOSITORY_MAINTENANCE.md` for the existing stable-snapshot
  and deletion rules.

## Cutover

1. Implement the native consumer behind an explicit ingest-source setting.
   Keep TAP as the default until the remaining steps pass.
2. Run the native consumer against an isolated staging database copied from
   staging. Compare record URIs/CIDs, collection/save counts, follow/favourite
   edges, account removals, and identity changes with TAP. Test a restart and a
   forced connection drop.
3. Switch Mac mini staging to Jetstream, then verify create/update/delete,
   account deletion and relogin backfill, and sync divergence using test DIDs.
4. Capture a production database backup, start from a cursor overlapping the
   still-running TAP stream (currently one hour of overlap, after confirming
   TAP's relay cursor is current), and stop TAP only after the new consumer is
   caught up. Keep the TAP image and database tables for rollback during the
   soak.

Hosted Jetstream's live WebSocket needs no API key, but recovery beyond its
lookback window uses the metered archive and requires a Jetstream API key. The
replacement must not silently clamp an old cursor to the live tip. The official
Go SDK supports archive-to-live replay but currently requires Go 1.26.6, so
adopting it also changes the appview toolchain and dependencies.

Jetstream provides decoded JSON, not TAP's verified repository proof. PDS
reconciliation can detect and repair state differences, but cannot provide the
same event-level signature and MST verification. This is an explicit integrity
trade-off in exchange for much lower ingress bandwidth and independent relay
transport.
