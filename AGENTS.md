# Currents — Claude Code Instructions

## Project overview

Currents is an open-source Pinterest alternative on the AT Protocol. Users save images into Collections. Two services:

- **`appview/`** — Go HTTP server, AT Protocol OAuth (indigo SDK), PostgreSQL + pgvector
- **`inference/`** — Python FastAPI server, image/text embeddings via SigLIP2 (`google/siglip2-base-patch16-naflex`)

## Coding philosophy

Code minimalism is a core pillar of this project. Write the least code that correctly solves the problem. No abstractions for their own sake, no extra configurability, no defensive handling of cases that can't happen. If something can be deleted without breaking behavior, delete it (always ask before doing it).

## Key conventions

- The inference server targets Apple Silicon (`DEVICE = "mps"`). When editing, preserve MPS/CPU fallback compatibility — avoid CUDA-only APIs.
- Embeddings from `/embed/image` and `/embed/text` share the same vector space (multimodal retrieval).
- The Go appview uses PostgreSQL with the pgvector extension for storing and querying embeddings.
- The appview indexes AT Protocol records via a TAP WebSocket listener (`appview/tap.go`). TAP is configured in docker-compose to filter `is.currents.*` collections and signal on `is.currents.actor.profile`. The `collection`, `save`, and `user` tables are populated by this listener, not by the HTTP handlers.
- The `collection` and `save` tables have no FK constraints on `author_did` or `collection_uri` — events arrive for all network users and ordering isn't guaranteed.
- **Visual identity** deduplicates images across saves. Every save gets a `visual_identity_id` FK pointing to a `visual_identity` row. Resolution happens in `appview/tap.go` (`handleSaveUpsert`): resave-of-known-save → same CID already in DB → novel image (fetch blob + inference). The inference server returns both the SigLIP2 embedding and image metadata used to populate width, height, and dominant-color palette. `visual_identity` holds the canonical blob reference, embedding (HNSW-indexed), dominant-color palette, and save count. `save_count` is maintained by a DB trigger; canonical re-election happens in Go (`DeleteSave`). If the inference server is unreachable, `visual_identity_id` is left NULL and the event is acked — backfill manually later.
- **Actor profiles** are indexed into the `user` table from two sources: `is.currents.actor.profile` TAP events (all active Currents users) and the OAuth callback (`ensureUserProfile` in `appview/auth.go` for the logged-in user). Avatar URLs are stored as `{CDN_URL}/img/{did}/{cid}` — routed through the appview's image proxy. The `user` table upserts on conflict, so profiles stay current.
- **XRPC endpoints** live in `appview/xrpc.go`. The appview's service DID is `did:web:{hostname}` (served at `/.well-known/did.json`). `getActorCollections`, `getProfile`, `getCollectionSaves`, `getSaves`, `searchSaves`, `getRelatedSaves`, and `getFeed` use optional auth — session cookie for the first-party web client, Bearer JWT for PDS-proxied calls (`atproto-proxy`), unauthenticated otherwise. The `CDN_URL` env var sets the base URL embedded in image URLs in XRPC responses. Image URLs are always built from the save's own `author_did` + `pds_blob_cid` — the visual identity is internal only and never surfaced in XRPC responses.
- **Save flow**: saves are AT Protocol records written to the user's PDS, then indexed asynchronously by the TAP listener. The frontend never writes to the appview DB directly. For new saves from the browser extension, the extension uploads the blob to the user's PDS and calls `com.atproto.repo.createRecord`. For resaves (saving an image that already exists on another user's PDS), the frontend calls `POST /resave` on the appview, which fetches the blob from the original author's PDS, uploads it to the viewer's PDS, and creates the record — avoiding CORS issues with cross-origin blob fetches. In both cases the TAP listener picks up the new record and populates the `save` table. Viewer save state in XRPC responses (`viewer.saves`) is hydrated by matching `pds_blob_cid` (content-addressed, so the same image bytes produce the same CID regardless of which PDS hosts the blob).
- **Starred collections** are stored in the `starred_collection` table (viewer_did + collection_uri PK). The `getActorCollections` and `getCollectionSaves` endpoints hydrate viewer state (`starred`) when the request is authenticated.
- **`searchSaves`** embeds the query text via the inference server (`/embed/text`) and runs pgvector ANN search (`<=>` cosine distance) over `visual_identity.embedding`. Offset-based pagination.
- **`getFeed`** returns a discovery feed. Global mode (unauthenticated or `personalized=0`): popular+recent saves filtered by `visual_identity.save_count` and a 30-day recency window, with iterative fallback if results are sparse. Personalized mode (`personalized=0–1`): the viewer's top collections are ranked by a time-decayed importance score (PinSage formula), each collection's precomputed `canonical_embedding` (medoid of its saves' embeddings) drives a pgvector ANN search, and results are blended with the global pool proportionally. The `canonical_embedding` on each collection row is updated in the background by a debounced goroutine in `tap.go` (30 s delay after save arrival).

## Running locally

```bash
# Full stack (appview + db)
SESSION_SECRET=<secret> docker compose up --build

# Inference server only
cd inference && uvicorn main:app --reload
```

## Common tasks

- **Add a new inference endpoint**: edit `inference/main.py`, follow the existing pattern (queue-based batching for text, direct executor for images).
- **Add a DB migration**: add a new numbered file to `appview/migrations/` (e.g. `006_*.sql` + `006_*.down.sql`). golang-migrate applies them in order on startup.
- **Handle a new TAP collection**: add a case to the `switch ev.Collection` in `handleTapRecord` in `appview/tap.go`, define a record struct, and add store methods in `appview/pgstore.go`.
- **Add a new XRPC endpoint**: define the lexicon in `lexicons/so/currents/`, add the handler to `appview/xrpc.go`, and register the route in `appview/main.go`. Use `srv.AuthValidator.Middleware(handler, true)` for mandatory auth, or `s.optionalAuth(r)` inside the handler for optional auth.
- **Change the embedding model**: update `CHECKPOINT` in `inference/main.py`, verify the processor API is compatible, and update the `VECTOR(768)` dimension in the migration if the output size changes.
- **Backfill missing visual identities**: query `SELECT uri FROM save WHERE visual_identity_id IS NULL` and re-process those saves through the inference pipeline.

## Frontend (`frontend/`) and Browser Extension (`extension/`)

SvelteKit app in frontend/ — TypeScript, npm, Tailwind CSS, ESLint, Prettier.
Svelte + TypeScript in extension/ — built with Vite and WXT, bundled as a browser extension.

**Svelte MCP tools** (available via the `svelte` MCP server):
- `list-sections` — browse Svelte/SvelteKit docs
- `get-documentation` — fetch full doc content
- `svelte-autofixer` — analyze and fix Svelte component issues
- `playground-link` — generate a Svelte Playground link

Always use the Svelte MCP server when doing frontend work: fetch relevant docs before writing components, and run `svelte-autofixer` after to confirm no issues remain.
