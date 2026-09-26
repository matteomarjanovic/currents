# Repository integrity and recovery

Currents writes user content to the user's PDS. TAP mirrors it asynchronously;
missing appview rows are not proof that a PDS record was deleted. Maintenance
repairs surviving records on the PDS and lets TAP update the appview normally.
It does not recreate deleted records or recover missing image blobs.

## Orphan recovery

The appview runs a recovery pass at startup and every 24 hours. It uses the
index to find accounts with broken collection references, then reads every
collection and save from that account's PDS, including all pagination pages.
The repository commit must stay unchanged throughout enumeration. A failed,
partial or changing read defers the account; an unavailable PDS is never
interpreted as an empty repository.

Recovery follows two rules, in this order:

1. A section whose parent collection is absent becomes a root collection by
   removing only its `parent` field.
2. A save whose collection is absent becomes Profile → Unsorted by removing
   only its `collection` field. Saves in a surviving orphaned section remain
   in that section, which rule 1 makes accessible.

Both rules keep the existing record URI, timestamps, images, alt text, notes,
attribution, self-labels, resave reference and unknown fields. No duplicate
records or blob uploads are needed. References to another account, malformed
references and missing `resaveOf` sources are outside this repair's scope.

The `orphan_record` table records the first confirmed observation of a broken
reference. It must persist for at least 24 hours before automatic repair; the
original record's creation time does not count. Changing the target resets
that grace period. A later complete scan clears observations that are no
longer orphaned; each successful write batch clears its observations right
away. Successful writes are harmless to repeat: the next scan sees the
repaired records and has nothing to change.

Accounts with active import jobs/items, collection deletion jobs, or a PDS
account wipe are skipped, including in an operator-requested immediate pass.
Recovery requires a usable existing OAuth session for writes. Without one,
observations remain pending until the user reconnects and a later pass runs.
No tokens are requested from the user or stored in repair reports.

Writes use atomic `applyWrites` batches of at most 200 operations with
`swapCommit`. Any concurrent repository change, including a parent returning,
invalidates the batch. On conflict, rate limit or network failure, stop and
re-read on the next pass; never replay a stale record payload. This protects
external clients as well as Currents. Currents' own collection/save writes,
import writes, account deletion and maintenance also share a per-account
PostgreSQL advisory lock across processes.

## Reliable collection deletion

`DELETE /collection/{id}` (and its `/api` alias) now returns **202 Accepted**
after persisting a `collection_delete_job`. The UI says deletion is underway
and removes the collection from the current view. It does not claim every PDS
record is already gone.

The maintenance worker checks jobs at startup and approximately every minute.
It enumerates the PDS, cancels imports targeting the collection and its
sections, and deletes saves first, sections next, and the parent last. This
avoids hiding sections while a deletion is stalled. Enumeration never relies
on TAP having already indexed all the imported records.

Jobs survive restarts, rate limits, lost OAuth sessions and failed batches.
The next attempt enumerates what remains. A completed job stays as a tombstone
until TAP has removed its indexed collection/sections. The job's `error`
column records the most recent failure; logs record completion. Account
deletion removes its repair observations and collection deletion jobs.

The import listing stage cannot append items to a cancelled job or set it back
to running. Claimed items are rechecked before writing. Repair skips pending
deletions instead of promoting records the user explicitly asked to remove.

## Destination and embedding safeguards

Collection listings used by selectors and Quick Save recommendations exclude
sections without an indexed root parent and collections pending deletion.
Recommendations also require a save and a canonical embedding. Save/upload,
resave, collection-parent edits and import destinations validate ownership and
hierarchy against the PDS, so a lagging index does not reject a newly created
collection and an orphan cannot accept invisible saves. Invalid destinations
are rejected before uploading image bytes.

Save deletions and moves invalidate the affected collection embeddings in the
same DB transaction, through a trigger. TAP schedules recomputation for the
old and new destinations; an empty collection's embedding becomes NULL.
Recomputation locks the collection row so a concurrent deletion cannot be
followed by an obsolete medoid being written back. The daily maintenance pass
also recomputes missing medoids in case a restart lost a debounce timer.
Migration 054 clears historical embeddings on collections without embedded
saves. This does not call inference or change any PDS content.

## Operating and checking a repair

Use the deployed appview environment, including its database and OAuth client
configuration. As with other appview commands, startup applies pending schema
migrations; `--dry-run` prevents repair writes/observations, not startup
migrations. Take the normal database backup before deploying a new migration.

```sh
# Inspect one account; nothing is rewritten or recorded as an observation.
appview repair-orphans --did did:plc:example --dry-run

# Record observations and repair only references observed at least 24h ago.
appview repair-orphans --did did:plc:example

# For an independently verified historical incident, preview the immediate plan.
appview repair-orphans --did did:plc:example --now --dry-run

# Apply that immediate plan. --now is refused without an explicit --did.
appview repair-orphans --did did:plc:example --now
```

Each candidate log includes its URI, missing target, field to remove and grace
eligibility. The final report counts accounts, candidates, eligible changes
and confirmed updates. Errors return a nonzero exit code; earlier batches may
have succeeded, so re-enumerate rather than assuming all-or-nothing across
batches. A batch whose response is lost is safely resolved by the next scan.

Before a targeted recovery, retain the public collection and save records.
Afterward compare:

- The same collection/save URIs and save CIDs remain (when only sections were
  promoted); only the intended reference fields changed.
- PDS records and indexed collection counts converge after TAP processes events.
- The profile shows the promoted collections and their existing saves, or
  Unsorted shows saves whose actual collection was missing.
- Another dry run reports zero candidates; imports remain stopped.

Do not test destructive deletion by deleting a real user's recovered content.
The regression suite uses a paginated mock PDS and an isolated PostgreSQL
`*_test` database to exercise partial failures, commit conflicts, import
cancellation, retries, grace periods, stale embeddings and metadata preservation.

```sh
cd appview
TEST_DATABASE_URL=postgres://appview:appview@localhost:55432/appview_test go test ./...
```
