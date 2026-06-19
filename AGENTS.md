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
- **Save flow**: saves are AT Protocol records written to the user's PDS, then indexed asynchronously by the TAP listener. The frontend never writes to the appview DB directly. For new saves, both the web uploader and the browser-extension clipper POST the image to the appview's `POST /save` endpoint (`CreateSave`), which uploads the blob to the user's PDS and writes the record. For resaves (saving an image that already exists on another user's PDS), the frontend calls `POST /resave` on the appview, which fetches the blob from the original author's PDS, uploads it to the viewer's PDS, and creates the record — avoiding CORS issues with cross-origin blob fetches. In both cases the TAP listener picks up the new record and populates the `save` table. Viewer save state in XRPC responses (`viewer.saves`) is hydrated by matching `pds_blob_cid` (content-addressed, so the same image bytes produce the same CID regardless of which PDS hosts the blob).
- **Unsorted saves**: a save's `collection` is optional. A save with no collection is stored with `collection_uri = ''` (empty-string sentinel, not NULL — there's no migration) and is "unsorted": it surfaces only on the author's profile via `is.currents.feed.getUnsortedSaves` (the profile **Unsorted** tab), never inside any collection (collection-scoped queries join on real URIs, so `''` is naturally excluded from save counts / canonical embeddings / the personalized feed). The "Profile" row in the collection selector saves here. An image can be in a collection *and* unsorted simultaneously (independent save records, same blob). Unsorted is populated only by **explicit** saves to the profile — unsaving (removing a save from a collection or from the profile) deletes that save record outright (`DELETE /save/{rkey}`); there is no move-to-unsorted fallback.
- **Starred collections** are stored in the `starred_collection` table (viewer_did + collection_uri PK). The `getActorCollections` and `getCollectionSaves` endpoints hydrate viewer state (`starred`) when the request is authenticated.
- **`searchSaves`** embeds the query text via the inference server (`/embed/text`) and runs pgvector ANN search (`<=>` cosine distance) over `visual_identity.embedding`. Offset-based pagination.
- **`getFeed`** returns a discovery feed. Global mode (unauthenticated or `personalized=0`): popular+recent saves filtered by `visual_identity.save_count` and a 30-day recency window, with iterative fallback if results are sparse. Personalized mode (`personalized=0–1`): the viewer's top collections are ranked by a time-decayed importance score (PinSage formula), each collection's precomputed `canonical_embedding` (medoid of its saves' embeddings) drives a pgvector ANN search, and results are blended with the global pool proportionally. The `canonical_embedding` on each collection row is updated in the background by a debounced goroutine in `tap.go` (30 s delay after save arrival).
- **Feature announcements** ("new feature" dots) are per-user and server-backed so they follow the user across browsers/devices. The `seen_feature` table (`viewer_did` + `feature_key` PK) records what each user has dismissed; `feature_key` is an arbitrary string owned by the frontend, so announcing a new feature needs **no backend change**. Endpoints: `GET /api/features/seen` → `{seen:[...]}` and `POST /api/features/seen/{key}` (both session-cookie auth, in `appview/features.go`). The web client mirrors this in `frontend/src/lib/stores/features.svelte.ts`: declare a `FEATURE_*` key constant, add it to `ACTIVE_ANNOUNCEMENTS` (drives the aggregate avatar dot in `top-bar.svelte`), show an indicator where `!isFeatureSeen(key)`, and call `markFeatureSeen(key)` when the user engages (e.g. opens the feature's page). `loadSeenFeatures()` unions into the set (flags are append-only); indicators are gated on `features.loaded` to avoid flashing.
- **Moderation** is built around an in-process atproto labeler (`appview/labeler_*.go`) that signs `com.atproto.label.defs#label` records and serves them via standard `subscribeLabels` / `queryLabels` endpoints. Three-axis auto-classification (NSFW / violence / AI-generated) happens inside the existing `/embed/image` SigLIP2 pass — no extra GPU work. Scores cross thresholds in `appview/moderation.go` (`ClassifyHarmScore`, `IsAIGenerated`) to drive a `currents-{nsfw,violence}-suspected` or `currents-ai-generated` label plus an entry in `review_item`. State is keyed by **exact blob CID** (`blob_moderation_state.blob_cid` = `save.pds_blob_cid`), NOT by visual identity — exact-bytes dedup is what we want for moderation. Read queries (search, related, feed, by-uri, collection saves) all exclude `harm_state='blocked'` blobs; the `<LabeledMedia>` Svelte wrapper blurs/badges/hides based on the labels XRPC hands back, resolved against the viewer's **per-user moderation preferences**. Those preferences are server-backed (so they follow the user across browsers/devices/mobile): the `moderation_pref` table holds one row per user (defaults: adult axes `blur`, AI-generated `show`), exposed via `GET`/`PUT /api/moderation/prefs` (session-cookie auth, in `appview/moderation_prefs.go`) and mirrored client-side in `frontend/src/lib/stores/moderation-prefs.svelte.ts` (loaded once on mount, optimistic `PUT` on each change). Absence of a row means the user is on the defaults — no migration-time provisioning of existing users. Admin endpoints under `/api/admin/*` (gated by the `moderator` table) drive the human review workflow at `/admin/queue`. The labeler's DID, service endpoint, and signed-label format are documented in `MODERATION.md`; deployment steps (DNS, keypair, service-record publishing) live in `MODERATION_DEPLOYMENT.md`.

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
- **Backfill moderation**: `appview backfill-moderation [--dry-run] [--limit N]` re-scores existing saves through `/classify/safety/embeddings`. Idempotent; honors Ctrl+C.
- **Tune moderation thresholds**: edit constants in `appview/moderation.go` (`ThresholdAutoSuspected`, `ThresholdReview`, `ThresholdAIGen`). Both live TAP and backfill share the ladder via `ClassifyHarmScore` / `IsAIGenerated`.
- **Add a custom label value**: declare it in `app.bsky.labeler.service` (see `MODERATION_DEPLOYMENT.md`), add constants in `appview/moderation.go`, and update the blur/badge sets in `frontend/src/lib/components/labeled-media.svelte`.
- **Announce a new feature** (one-time "new" dot): add a `FEATURE_*` key constant to `frontend/src/lib/stores/features.svelte.ts` and list it in `ACTIVE_ANNOUNCEMENTS`, render an indicator gated on `!isFeatureSeen(key)` where the feature is surfaced, and call `markFeatureSeen(key)` when the user engages with it. No DB/endpoint change needed — keys are free-form (the `seen_feature` table stores whatever the frontend posts). Remove the key from `ACTIVE_ANNOUNCEMENTS` once it's no longer newsworthy.

## Frontend (`frontend/`) and Browser Extension (`extension/`)

SvelteKit app in frontend/ — TypeScript, npm, Tailwind CSS, ESLint, Prettier.
Svelte + TypeScript in extension/ — built with Vite and WXT, bundled as a browser extension.

**Svelte MCP tools** (available via the `svelte` MCP server):
- `list-sections` — browse Svelte/SvelteKit docs
- `get-documentation` — fetch full doc content
- `svelte-autofixer` — analyze and fix Svelte component issues
- `playground-link` — generate a Svelte Playground link

Always use the Svelte MCP server when doing frontend work: fetch relevant docs before writing components, and run `svelte-autofixer` after to confirm no issues remain.
