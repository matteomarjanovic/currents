# appview

AT Protocol AppView — a Go HTTP server implementing atproto OAuth (via the [indigo](https://github.com/bluesky-social/indigo) SDK), backed by PostgreSQL.

## Prerequisites

- Go 1.24+
- PostgreSQL (or use Docker Compose — see below)

## Configuration

All options can be set via flags or environment variables.

| Env var | Flag | Required | Description |
|---|---|---|---|
| `APPVIEW_MODE` | `--mode` | No | Process mode: `all` or `repair` (default: `all`) |
| `SESSION_SECRET` | `--session-secret` | Yes | Random secret for signing session cookies |
| `DATABASE_URL` | `--database-url` | Yes | PostgreSQL DSN, e.g. `postgres://user:pass@host:5432/db?sslmode=disable` |
| `DB_MIN_CONNS` | `--db-min-conns` | No | Minimum PostgreSQL connections kept open (default: `4`) |
| `DB_MAX_CONNS` | `--db-max-conns` | No | Maximum PostgreSQL connections in the pool (default: `12`) |
| `DB_MAX_CONN_LIFETIME` | `--db-max-conn-lifetime` | No | Maximum PostgreSQL connection lifetime (default: `30m`) |
| `DB_MAX_CONN_IDLE_TIME` | `--db-max-conn-idle-time` | No | Maximum PostgreSQL connection idle time (default: `5m`) |
| `CLIENT_HOSTNAME` | `--hostname` | No | Public hostname (e.g. `example.com`). Omit for localhost dev mode |
| `CLIENT_SECRET_KEY` | `--client-secret-key` | No | P-256 private key in multibase encoding (confidential clients only) |
| `CLIENT_SECRET_KEY_ID` | `--client-secret-key-id` | No | Key ID for `CLIENT_SECRET_KEY` (default: `primary`) |
| `INFERENCE_URL` | `--inference-url` | No | Base URL of the inference FastAPI server (default: `http://localhost:8000`) |
| `CDN_URL` | `--cdn-url` | No | Base URL for image CDN used in XRPC responses. Defaults to `http://127.0.0.1:8080` in localhost mode or `https://<hostname>` in production |

The HTTP server also now uses explicit timeouts (`ReadHeaderTimeout=10s`, `ReadTimeout=30s`, `WriteTimeout=60s`, `IdleTimeout=60s`) instead of the Go defaults.

`APPVIEW_MODE=all` runs the HTTP server, TAP listener, and in-process background enrichment in a single process. `APPVIEW_MODE=repair` runs a one-shot backfill pass that processes unresolved saves and missing collection embeddings directly from the database, then exits.

## Running with Docker Compose (recommended)

The compose file at the repo root starts both the appview and a PostgreSQL 18 instance with pgvector.

```bash
# From the repo root
SESSION_SECRET=<random-string> docker compose up --build
```

The server will be available at http://localhost:8080.

To stop:
```bash
docker compose down
```

To stop and wipe the database volume:
```bash
docker compose down -v
```

## Running locally

1. Make sure PostgreSQL is running and create a database:
   ```bash
   createdb appview
   ```

2. From this directory:
   ```bash
   SESSION_SECRET=<random-string> \
   DATABASE_URL="postgres://localhost:5432/appview?sslmode=disable" \
   go run .
   ```

The server binds to `:8080`.

## OAuth modes

### Localhost / dev (default)

When `CLIENT_HOSTNAME` is not set, the server registers itself as a public client using `http://127.0.0.1:8080/oauth/callback` as the redirect URI. This works out of the box for local testing with any atproto PDS.

### Production (public client)

Set `CLIENT_HOSTNAME` to your domain:
```bash
CLIENT_HOSTNAME=example.com \
SESSION_SECRET=<random-string> \
DATABASE_URL=<dsn> \
go run .
```

The server will advertise `https://example.com/oauth-client-metadata.json` as its client ID.

### Production (confidential client)

Generate a P-256 key in multibase encoding and set `CLIENT_SECRET_KEY` alongside `CLIENT_HOSTNAME`:
```bash
CLIENT_HOSTNAME=example.com \
CLIENT_SECRET_KEY=<multibase-p256-key> \
SESSION_SECRET=<random-string> \
DATABASE_URL=<dsn> \
go run .
```

## Visual identity

When a new save event arrives via TAP, the appview persists the save immediately and then resolves `visual_identity_id` in two phases:

1. **Resave shortcut** — if `resaveOf` points to a save already in the DB, reuse its `visual_identity_id` and `quality_score` directly (no blob fetch).
2. **CID match** — if the blob CID already appears in another save row, reuse that row's `visual_identity_id` and `quality_score` (same CID = same pixels).
3. **Novel image** — launch in-process async enrichment keyed by blob CID. A bounded goroutine fetches the blob from a candidate author's PDS, calls the inference server for both the embedding and image metadata, does the nearest-neighbor search against existing visual identities (cosine distance ≤ 0.02 = match), and then links every matching save row for that CID.

Collection canonical embeddings are recomputed with an in-memory debounce so bulk imports do not run medoid calculation on every save.

If the inference server is unreachable, the save is still stored and enrichment falls back to repair/backfill later. Missing identities can still be inspected with `SELECT uri FROM save WHERE visual_identity_id IS NULL`.

## Background Operations

`GET /debug/background` returns live backlog metrics from PostgreSQL, including:

- saves missing `visual_identity_id`
- distinct blob CIDs still missing enrichment
- collections whose `canonical_embedding` is still missing even though resolved save embeddings exist
- oldest unresolved save age

There is also a tiny built-in monitoring page at `/ops` that polls `/debug/background` and renders the same metrics in the appview web UI.

Run a one-shot repair/backfill pass with:

```bash
DATABASE_URL=<dsn> APPVIEW_MODE=repair go run .
```

That pass processes distinct unresolved blob CIDs directly from `save`, then recomputes collection embeddings for collections whose `canonical_embedding` is still missing even though resolved save embeddings exist.

The `visual_identity` table stores the canonical blob reference (best-quality source), the embedding (HNSW-indexed for fast ANN search), a dominant-color palette for placeholder rendering, and a `save_count` maintained by a DB trigger.

## Endpoints

### Web UI (session cookie auth)

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/` | — | Home |
| `GET` | `/oauth/login` | — | Login page |
| `POST` | `/oauth/login` | — | Start OAuth flow |
| `GET` | `/oauth/callback` | — | OAuth redirect callback |
| `GET` | `/oauth/logout` | — | Logout and revoke tokens |
| `GET` | `/oauth-client-metadata.json` | — | OAuth client metadata |
| `GET` | `/oauth/jwks.json` | — | JWKS (confidential clients only) |
| `GET` | `/collection` | Required | List and manage collections |
| `POST` | `/collection` | Required | Create collection |
| `GET` | `/collection/{id}` | Required | Get collection |
| `POST` | `/collection/{id}` | Required | Update collection |
| `POST` | `/collection/{id}/delete` | Required | Delete collection |
| `GET` | `/save` | Required | List and manage saves |
| `POST` | `/save` | Required | Create save |
| `GET` | `/save/{id}` | Required | Get save |
| `POST` | `/save/{id}` | Required | Update save |
| `POST` | `/save/{id}/delete` | Required | Delete save |
| `GET` | `/feed` | — | Feed explorer page (try getFeed, no auth required) |
| `GET` | `/img/{did}/{cid}` | — | Blob image proxy (long-cache) |

### XRPC (AT Protocol)

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/.well-known/did.json` | — | Service DID document (`did:web:<hostname>`) |
| `GET` | `/xrpc/is.currents.actor.getProfile` | Optional | Detailed actor profile by DID or handle |
| `GET` | `/xrpc/is.currents.feed.getActorCollections` | Optional | Any actor's collections; viewer state included when authenticated |
| `GET` | `/xrpc/is.currents.feed.getCollectionSaves` | Optional | Collection view plus saves within that collection; viewer state included when authenticated |
| `GET` | `/xrpc/is.currents.feed.getSaves` | Optional | Hydrated save views by AT-URI; viewer state included when authenticated |
| `GET` | `/xrpc/is.currents.feed.searchSaves` | Optional | Semantic image search via text query (SigLIP2 embedding, requires inference server) |
| `GET` | `/xrpc/is.currents.feed.getRelatedSaves` | Optional | Visually similar saves for a source save; viewer state included when authenticated |
| `GET` | `/xrpc/is.currents.feed.getFeed` | Optional | Discovery feed — global (popular+recent), personalized, or serendipitous; `personalized` param -1–1 |
