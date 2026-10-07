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

The native appview consumer is now implemented behind `INGEST_SOURCE=jetstream`;
the default remains `tap`. Its cursor, repo tracking, opt-outs and PDS backfill
are durable PostgreSQL state. An isolated staging database subscribed to the
real v2 endpoint, caught up from a one-hour lookback, indexed a 53-save repo
with exactly the same save and collection URIs as TAP, and resumed from its
saved cursor after a process restart. A 13-hour run on the normal Mac mini
staging appview stayed at live witness time. The staging TAP container has
then been scaled to zero with `TAP_SCALE=0` for a bounded source-only test:
appview indexed new saves while TAP's mirror count stayed fixed, including a
save URI present only in appview. TAP was restored afterward, and production
has not switched.

Database-backed tests now cover account deletion with retained PDS records,
relogin and full PDS restoration, sync divergence, and replay of older and
newer commits. A `RepoNotFound` remains an observation and retry: the native
consumer never deletes the old appview rows on that result.

Raw TAP/appview table counts are not a sufficient convergence check. Staging
has two long-standing TAP `error` repos whose current PDS says `RepoNotFound`
but whose old appview rows still hold 6,352 saves. Small active-repo graph
count differences existed before this switch: some PDS follow records have
duplicate subjects, one TAP collection URI is no longer on the PDS, and one
PDS favourite was missing from appview. A targeted staging PDS reconciliation
restored that favourite without changing the owner's save or collection count.
These historical discrepancies need a deliberate PDS-authoritative audit and
missing-repo policy before production cutover.

`appview queue-jetstream-audit --did DID` and `--all` report the number of
tracked repos and existing saves they would scan. Add `--apply` to enqueue
the selected scope; the native worker processes it durably. A targeted DID
gets a full record refresh. `--all` performs a membership audit: it adds
missing saves and removes stale ones without rewriting saves already indexed.
It therefore does not prove that every historical save's mutable fields match
the PDS. The all-repo scope is roughly a million staging saves, so preview it
and plan capacity before queuing it. An explicit PDS `RepoNotFound` records a
durable `missing_since` observation and retries without removing old appview
rows. This preserves data until the missing-repo policy and grace period are
settled.

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

The native code can be delivered before switching ingestion. Its default is
TAP. This branch intentionally leaves `docker-compose.scaleway.yml` unchanged:
merging it deploys appview and applies additive migrations but does not set
`INGEST_SOURCE` or stop TAP. Production Compose configuration changes belong
to the later cutover step.

1. Merge the dormant appview code with TAP still active and check the normal
   deployment and migrations. The controlled lifecycle tests and Mac mini
   source-only soak are complete; staging has been restored to TAP.
2. Settle the policy for a repeatedly missing PDS repo. The current safe
   behavior records the error and preserves the rows. Configure a Jetstream
   archive API key for restart recovery beyond the live lookback window, then
   verify archive replay from a saved cursor on staging.
3. Update the production Compose file and environment as a separate cutover
   change. Capture a production database backup and start from a cursor overlapping the
   still-running TAP stream (currently one hour of overlap, after confirming
   TAP's relay cursor is current), and switch to `INGEST_SOURCE=jetstream`.
   Set `TAP_SCALE=0` only after the new consumer is caught up. Keep the TAP
   image and database tables for rollback during the soak.

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
