# appview

AT Protocol AppView — a Go HTTP server implementing atproto OAuth (via the [indigo](https://github.com/bluesky-social/indigo) SDK), backed by PostgreSQL.

## Prerequisites

- Go 1.24+
- PostgreSQL (or use Docker Compose — see below)

## Configuration

All options can be set via flags or environment variables.

| Env var | Flag | Required | Description |
|---|---|---|---|
| `SESSION_SECRET` | `--session-secret` | Yes | Random secret for signing session cookies |
| `DATABASE_URL` | `--database-url` | Yes | PostgreSQL DSN, e.g. `postgres://user:pass@host:5432/db?sslmode=disable` |
| `CLIENT_HOSTNAME` | `--hostname` | No | Public hostname (e.g. `example.com`). Omit for localhost dev mode |
| `CLIENT_SECRET_KEY` | `--client-secret-key` | No | P-256 private key in multibase encoding (confidential clients only) |
| `CLIENT_SECRET_KEY_ID` | `--client-secret-key-id` | No | Key ID for `CLIENT_SECRET_KEY` (default: `primary`) |
| `INFERENCE_URL` | `--inference-url` | No | Base URL of the inference FastAPI server (default: `http://localhost:8000`) |
| `CDN_URL` | `--cdn-url` | No | Base URL for image CDN used in XRPC responses. Defaults to `http://127.0.0.1:8080` in localhost mode or `https://<hostname>` in production |

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

When a new save event arrives via TAP, the appview assigns it a `visual_identity_id` using a three-step resolution:

1. **Resave shortcut** — if `resaveOf` points to a save already in the DB, reuse its `visual_identity_id` and `quality_score` directly (no blob fetch).
2. **CID match** — if the blob CID already appears in another save row, reuse that row's `visual_identity_id` and `quality_score` (same CID = same pixels).
3. **Novel image** — fetch the blob from the author's PDS, call `POST /analyze/image` on the inference server to get the SigLIP2 embedding + dominant colors + dimensions, then do a nearest-neighbor search against existing visual identities (cosine distance ≤ 0.02 = match). If a match is found the save links to it; otherwise a new `visual_identity` row is created.

If the inference server is unreachable, the save is stored with `visual_identity_id = NULL` and the TAP event is still acked. Missing identities can be backfilled later: `SELECT uri FROM save WHERE visual_identity_id IS NULL`.

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
| `GET` | `/collections` | Required | List and manage collections |
| `POST` | `/collections` | Required | Create collection |
| `GET` | `/collections/{id}` | Required | Get collection |
| `POST` | `/collections/{id}` | Required | Update collection |
| `POST` | `/collections/{id}/delete` | Required | Delete collection |
| `GET` | `/saves` | Required | List and manage saves |
| `POST` | `/saves` | Required | Create save |
| `GET` | `/saves/{id}` | Required | Get save |
| `POST` | `/saves/{id}` | Required | Update save |
| `POST` | `/saves/{id}/delete` | Required | Delete save |
| `GET` | `/feed` | — | Feed explorer page (try getFeed, no auth required) |
| `GET` | `/img/{did}/{cid}` | — | Blob image proxy (long-cache) |

### XRPC (AT Protocol)

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/.well-known/did.json` | — | Service DID document (`did:web:<hostname>`) |
| `GET` | `/xrpc/is.currents.feed.getCollections` | Required (Bearer service JWT) | Authenticated user's own collections with preview images |
| `GET` | `/xrpc/is.currents.feed.getActorCollections` | Optional | Any actor's collections; viewer state included when authenticated |
| `GET` | `/xrpc/is.currents.feed.getSaves` | Optional | Saves within a collection; viewer state (`resaved`) included when authenticated |
| `GET` | `/xrpc/is.currents.feed.searchSaves` | Optional | Semantic image search via text query (SigLIP2 embedding, requires inference server) |
| `GET` | `/xrpc/is.currents.feed.getFeed` | Optional | Discovery feed — global (popular+recent) or personalized by viewer's collection interests; `personalized` param 0–1 |
