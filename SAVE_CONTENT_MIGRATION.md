# Save Content Migration Guide

This rollout changes the save record shape from a top-level `image` field to an open-union `content` field, and changes XRPC save responses to return `content` instead of flat image fields.

## What Changes

- Save records written to user PDSes now use `content`.
- Appview/XRPC save responses now return `content`, with image saves using `is.currents.content.defs#imageView`.
- The appview DB stores `content_nsid` and `resave_of_cid` so it can serve the new shape and correct `resaveOf` strong refs.
- The one-off `migrate-save-content` command rewrites existing save records in place with `com.atproto.repo.putRecord`.

## Safe Rollout Order

1. Publish the updated lexicons.
2. Take a database backup and keep a copy of any existing OAuth session data.
3. Deploy the appview and frontend from the same commit.
4. Verify that new saves created after deploy are being written successfully.
5. Run the migration in dry-run mode on a small sample.
6. Run the full migration.
7. Verify that `resave_of_cid` is populated and that spot-checked repo records now contain `content`.

Do not stagger frontend and appview deployments. The frontend now expects `saveView.content`, and the appview now returns it.

## Why This Order Works

- The new appview can read both legacy save records with `image` and migrated save records with `content`.
- Save writes now send `Validate: false` to the PDS for `is.currents.feed.save`, so stale lexicon caches do not block the cutover.
- The migration is resumable through a progress file.

## Dry Run

Run a narrow dry-run first:

```bash
cd appview
go run . \
  --mode migrate-save-content \
  --database-url "$DATABASE_URL" \
  --migration-dry-run \
  --migration-limit 20
```

Useful filters:

- `--migration-author did:plc:...`
- `--migration-uri at://did:plc:.../is.currents.feed.save/...`

The dry-run fetches records and reports what would change, but it does not PUT anything and does not write the progress file.

The migration command reuses the appview's stored OAuth sessions from `oauth_sessions`. Every author whose saves you want to rewrite must have at least one live local session. If the command reports missing or dead sessions for a DID, log in as that account once and rerun.

## Full Migration

Example:

```bash
cd appview
go run . \
  --mode migrate-save-content \
  --database-url "$DATABASE_URL" \
  --migration-progress-file save-content-migration-progress.json \
  --migration-global-rate 1 \
  --migration-per-account-rate 0.5
```

Behavior:

- Pass A rewrites legacy records from `image` to `content`.
- Pass B refreshes `resaveOf.cid` after any referenced record CID churn.
- The progress file is updated after each successful record, so reruns continue where the last run stopped.

## Verification

After the migration completes, these checks should be clean:

```sql
SELECT count(*)
FROM save
WHERE content_nsid <> 'is.currents.content.image';

SELECT count(*)
FROM save
WHERE resave_of_uri IS NOT NULL
  AND COALESCE(resave_of_cid, '') = '';
```

For repo-level spot checks, fetch a few records directly from the authors' PDSes and confirm:

- the top-level `image` field is gone
- the record has a `content` object
- image content carries `$type: "is.currents.content.image"`
- resaves have an up-to-date `resaveOf.cid`

Also spot-check the frontend on:

- a normal image save
- a resave
- a save detail page
- feed/search results

## Reruns And Recovery

- If the command exits with failures, fix the blocking issue and rerun the same command with the same progress file.
- If you used filters for a partial run, do a final unfiltered run before declaring the migration complete.
- Keep the progress file until post-migration verification is done.

## Rollback Constraint

Once repo records have been rewritten to `content`, do not roll the appview/frontend back to code that only understands legacy `image` saves. At that point the safe path is forward: fix the new deployment and rerun the migration if needed.