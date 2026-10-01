# Jetstream shadow probe

This standalone command observes Jetstream v2 without changing Currents data.
It subscribes to `is.currents.*` commits and the account, identity, and sync
markers that Jetstream sends with a collection filter. It prints each event's
witness lag and a final count, byte total, and cursor.

```sh
cd appview
go run ./cmd/jetstream-shadow --duration 2m
```

Use `--did` for one account and `--from` for a recent RFC3339 timestamp
when testing historical events.

Pass `--database-url` to compare each touched record URI and CID against
TAP's `repos` and `repo_records` tables in the shared PostgreSQL database.
The command forces its database sessions to read-only. `match` means TAP's
current record agrees; `tap-behind` means its repository revision has not
reached the event. A different CID after a newer revision is reported as
`changed-later-or-missing` because a later event could have changed it.
Untracked DIDs are reported separately. This is an event sample, not a full
repository audit. Fresh commits are checked after the `--settle` window
(15 seconds by default), so ordinary asynchronous TAP indexing is not
reported as a lasting delay. The command waits for those checks before
printing its final summary.

```sh
go run ./cmd/jetstream-shadow --database-url '<read-only PostgreSQL DSN>' \
  --duration 10m
```

The `--from` timestamp uses Jetstream's live WebSocket lookback window
(currently 36 hours on Bluesky's hosted service). To resume a bounded
observation, pass the final report's Jetstream sequence as `--cursor`.
The command has no persistent cursor and never handles records for the appview;
it is only a shadow measurement tool. Historical replay naturally reports a
large witness lag. Jetstream archive replay and account/sync folding remain
outside this prototype.
