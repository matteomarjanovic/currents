# Moderation

Currents' moderation system. This document is the technical reference: how content is classified, what labels mean, what the data flow looks like, where the code lives, and which database tables back which decisions.

For day-one ops (DNS, keypair, publishing the service record), see [`MODERATION_DEPLOYMENT.md`](MODERATION_DEPLOYMENT.md).

---

## Scope (v1)

- **Save-level only.** Account-level actions (suspending an actor) and CSAM are explicitly out of scope; they need their own plans.
- **Three classification axes:** NSFW, violence, AI-generated. The first two are *harm* axes (can blur, can take down, require human review); the third is a *provenance* axis (informational, auto-applied, never blurs, never gates content).
- **Atproto-native.** All labels are signed `com.atproto.label.defs#label` records served via the standard `subscribeLabels` / `queryLabels` endpoints. Bluesky clients can subscribe to the labeler and honor Currents-issued labels on Currents content viewed from Bluesky.
- **Defense in depth.** A taken-down save is filtered at the SQL layer of every save-returning query AND has a `!hide` label issued; clients that ignore the labeler still see nothing.

---

## Architecture

```
                          ┌──────────────────────┐     ┌────────────────────────────┐
                          │  inference (Python)  │     │  appview (Go)              │
TAP record arrives ─────► │  /embed/image        │ ──► │  processBlobEnrichment     │
(is.currents.feed.save)   │  · SigLIP2 forward   │     │   · UpsertBlobModeration…  │
                          │  · 3 ONNX heads      │     │   · ClassifyHarmScore →    │
                          │  · returns           │     │     IssueLabel + review    │
                          │    safety_scores?    │     │   · IsAIGenerated → label  │
                          └──────────────────────┘     └────────────┬───────────────┘
                                                                    │
                                                        ┌───────────▼────────────┐
                                                        │  Postgres              │
                                                        │   label                │
                                                        │   blob_moderation_state│
                                                        │   review_item          │
                                                        │   report               │
                                                        │   moderator            │
                                                        │   moderation_event     │
                                                        └───────────┬────────────┘
                                                                    │
                                                        ┌───────────▼─────────────────┐
                                                        │  labeler module (in-process)│
                                                        │   · signer (secp256k1)      │
                                                        │   · issuer (signs + INSERT) │
                                                        │   · subscribeLabels (WS)    │
                                                        │   · queryLabels (REST)      │
                                                        │   · createReport (REST)     │
                                                        │  served at                  │
                                                        │  https://moderation.<host>  │
                                                        └─────────────────────────────┘
                                                                    │
              ┌────────────────────────────────────────┬────────────┴──────────────────┐
              ▼                                        ▼                               ▼
        XRPC label hydration               Admin UI (/admin/queue)             Bluesky clients
        (every save returns                Moderator triages flags,            (subscribe to our
         a labels[] array)                 confirms or takes down              labeler DID)
              │
              ▼
        <LabeledMedia> Svelte
        wrapper renders blur,
        AI badge, or hides per
        user preference
```

---

## Three-axis classification

The inference server loads up to three ONNX classifier heads at startup (`NSFW_HEAD_ONNX`, `VIOLENCE_HEAD_ONNX`, `AIGEN_HEAD_ONNX`). Each is a small MLP trained on the SigLIP2 pooler output — the training notebooks live under [`moderation/`](moderation/). Heads run on L2-normalized 768-d features inside the same `/embed/image` request that produces the embedding; cost is microseconds per head.

The response includes an optional `safety_scores` field:

```json
{ "nsfw": 0.0–1.0, "violence": 0.0–1.0, "ai_generated": 0.0–1.0 }
```

`null` when no heads are loaded; present (with `0.0` for any unloaded axis) when at least one head is loaded.

### Threshold ladder

Defined in [`appview/moderation.go`](appview/moderation.go) and shared between live TAP enrichment and the backfill CLI via `ClassifyHarmScore(score)` and `IsAIGenerated(score)`:

| Axis | Score | Action |
|---|---|---|
| harm (nsfw or violence) | `score < 0.70` | nothing |
| harm | `0.70 ≤ score < 0.97` | `review_item` at normal priority, no label |
| harm | `score ≥ 0.97` | issue `currents-{nsfw,violence}-suspected` label + `review_item` at high priority |
| AI-gen | `score ≥ 0.90` | `SetAIGenerated(true)` + issue `currents-ai-generated-suspected` label + `review_item` at high priority |

The thresholds are deliberately conservative starting points; tune per-head when eval data exists. Changing them in one place automatically updates both pipelines.

All three axes share the same suspected → canonical pattern: the classifier proposes a suspected label, then either the author (via self-attestation, see below) or a moderator confirms it into a canonical label or dismisses/disputes it. AI-gen used to be auto-applied directly with no human in the loop; with the attestation flow it now goes through the same pattern as the harm axes.

### Self-attestation

When an auto-flag fires on a save, the save's author is notified via the **Notifications** dialog (avatar dropdown → Notifications). The author can:

- **Confirm** the flag — picks the canonical label (`porn`/`sexual`/`nudity` for NSFW, `graphic-media` for violence, `currents-ai-generated` for AI-gen). The system issues the canonical on every URI sharing the blob, negates the suspected label, resolves the `review_item`. Same end-state as a moderator confirm, just author-attributed (`actor_did = author DID`, `action = self_confirm` in `moderation_event`).
- **Dispute** the flag — the suspected label **stays up** (only moderators can clear flags). The `review_item` gets `disputed = true`, `disputed_at = now`, and a +50 priority bump so moderators see disputed items first. Moderator queue UI shows a `disputed by author` badge.
- **Do nothing** — moderator handles the `review_item` via the normal queue.

The dispute-doesn't-clear rule prevents bad actors from clearing their own flags by denying.

Backend endpoints (auth: session cookie; ownership check: `session_did == save.author_did`):
- `GET /api/me/attestations` — list pending review items for the author's saves
- `POST /api/me/attestations/{id}/confirm` body `{val}`
- `POST /api/me/attestations/{id}/dispute`

---

## Label vocabulary

| Label value | Source | Blurs | Severity | Effect in Currents |
|---|---|---|---|---|
| `porn`, `sexual`, `nudity` | global Bluesky vocab | media | alert (adultOnly) | Blur with click-to-reveal |
| `graphic-media` | global Bluesky vocab | media | alert | Blur with click-to-reveal |
| `!hide` | global Bluesky vocab | content | (built-in) | Replace with placeholder; backend also filters at SQL |
| `currents-nsfw-suspected` | Currents custom | media | alert (adultOnly) | Same as `porn` until human confirms / negates |
| `currents-violence-suspected` | Currents custom | media | alert | Same as `graphic-media` until human confirms / negates |
| `currents-ai-generated-suspected` | Currents custom | none | inform | Small corner badge — same visual treatment as the canonical; suspected vs canonical is a workflow distinction, invisible to viewers |
| `currents-ai-generated` | Currents custom | none | inform | Small corner badge |

The Currents-defined labels are declared in the `app.bsky.labeler.service` record (see [`MODERATION_DEPLOYMENT.md`](MODERATION_DEPLOYMENT.md)). The frontend's [`labeled-media.svelte`](frontend/src/lib/components/labeled-media.svelte) consumes them via the `labels[]` field on every `SaveView` and honors a user preference store in [`stores/moderation-prefs.svelte.ts`](frontend/src/lib/stores/moderation-prefs.svelte.ts).

**Per-viewer defaults differ by auth state** (`activePrefs()` in the prefs store). Logged-in users get the server-backed `moderation_pref` row (adult/violence axes default to `blur`, AI-generated `show`) and can change it in settings. **Logged-out visitors get the safest defaults — every adult/violence label `hide`, AI-generated `show` — and can't loosen them** (preferences live behind auth). Resolution reads `auth.user` reactively, so visibility flips on login/logout; this also covers the pre-auth window (defaults to hiding until login is confirmed).

---

## Blob-CID keying

Moderation state is keyed by the **exact content-addressed blob CID** (`save.pds_blob_cid`), not the perceptual `visual_identity_id`. This matters:

- Two visually similar but legally distinct images (e.g. before/after retouching) get separate `blob_moderation_state` rows. A takedown of one does not silently take down the other.
- All saves of the same bytes — whether the same user resaves to two collections, or another user resaves the content — share a single state row. A confirmation or takedown propagates to every URI in one operation.

The labeler issues per-URI labels (atproto labels target AT-URIs), but the `blob_moderation_state` is the canonical state; the `label.blob_cid` column is a denormalized cache that powers fast materialization onto new URIs.

---

## End-to-end data flow

### A new save arrives

1. **TAP listener** (`appview/tap.go`) receives a `is.currents.feed.save` record from the firehose and calls `handleSaveUpsert`.
2. Save row is upserted in three branches (resave-of-known, same-CID-already-linked, novel). After every successful upsert, `applyModerationAfterSaveUpsert` (`appview/moderation_tap.go`) runs:
   - **Materialize:** `GetActiveLabelsByBlobCID` returns any labels already issued for this blob; each is re-issued on the new save URI via `LabelerIssuer.IssueLabel`. So a resave of a known-NSFW image inherits the blur instantly, without re-running inference.
   - **Self-labels:** if the record contains a `com.atproto.label.defs#selfLabels` block, each `val` is issued as a labeler-signed label attributed to the author (`actor_did = save.AuthorDID`, `action = self_label`) **and blob-keyed**, so the warning joins the blob's active-label set. Existing copies of the same bytes are labeled by a retroactive fan-out (`propagateLabelToBlobSiblings`, once per `(blob, val)`); future copies inherit via the materialization above. A content warning is a fact about the bytes, not one record — but we never write a `labels` block into another user's repo; the labeler issues the propagated copies.
3. For novel images, `enqueueBlobEnrichment` schedules async work. Eventually `processBlobEnrichment` calls `Inference.EmbedImage`, which now includes `safety_scores` when heads are loaded.
4. `processSafetyScores` (`appview/moderation_tap.go`) walks the threshold ladder per axis:
   - Harm score → `HarmAutoSuspected`: issue the suspected label on the source URI + `UpsertReviewItem(priority=High)`
   - Harm score → `HarmReview`: `UpsertReviewItem(priority=Normal)` only (no label)
   - AI-gen score over threshold: `SetAIGenerated(true)` + issue `currents-ai-generated` on the source URI
5. The new label rows broadcast via `LabelerIssuer.broadcast` to any live `subscribeLabels` WebSocket clients.

The active-blob-labels query handles the concurrent-saves edge case: if multiple users save the same novel blob in the same instant, only one of them gets labelled by the enrichment path, but the others pick up the label via materialization on their next blob-CID lookup.

### A read request returns saves

All save-returning XRPC handlers (`getCollectionSaves`, `getSaves`, `searchSaves`, `getRelatedSaves`, `getFeed`) call `hydrateLabels(ctx, store, views)` (`appview/save_content.go`) after building views. That helper does a single batched `GetLabelsByURIs(uris)` query and projects to a compact `{src, val, cts}` shape.

The blocked-blob filter (`AND NOT EXISTS (SELECT 1 FROM blob_moderation_state b WHERE b.blob_cid = s.pds_blob_cid AND b.harm_state = 'blocked')`) is appended to the following queries in `appview/pgstore.go`:

- `searchSavesByEmbeddingPage`
- `getRelatedSavesPageByURI`
- `GetGlobalFeedSaves`
- `GetSavesPage` (collection saves)
- `GetSavesByURIs` (direct lookup)
- the `preview_blobs` subquery shared by the collection-listing queries (`GetActorCollectionsPage`, the authed variant, `GetCollectionByURI`, `SearchCollectionsPage`) — so a taken-down image never surfaces as a collection-card thumbnail either

So a taken-down save is invisible from discovery, from any collection's listing, from a direct URL hit, AND from collection-card previews — but its label is still queryable so Bluesky clients can also choose to hide it.

**Collection previews also honor per-viewer preferences.** Each `previewItem` returned by the collection-listing endpoints carries its blob's active label values (`GetActiveLabelsByBlobCIDs`), and the web [`collection-card.svelte`](frontend/src/lib/components/collection-card.svelte) applies the same `effectiveVisibility` logic as save tiles — dropping `hide`-preference previews and blurring `blur`-preference ones — via `effectiveVisibilityForVals`.

### A user reports a save

1. Frontend [`report-dialog.svelte`](frontend/src/lib/components/report-dialog.svelte) POSTs to `/xrpc/com.atproto.moderation.createReport` with session cookie.
2. `XRPCCreateReport` (`appview/labeler_report.go`) validates the subject is a `com.atproto.repo.strongRef`, inserts a `report` row, and (best-effort) looks up the save's blob CID to enqueue a `review_item` with `source = 'report'`.
3. The same endpoint accepts PDS-proxied calls (Bluesky clients sending `atproto-proxy: did:web:moderation.currents.is#atproto_labeler`), authenticated via the indigo `AuthValidator`.

### A moderator triages an item

1. Moderator signs into Currents normally; the `/admin/+layout.svelte` fetches `/api/me/role` and gates the route on the `moderator` table.
2. `/admin/queue` lists pending `review_item`s; `/admin/queue/[id]` shows scores, sibling saves (all URIs sharing the blob), active labels, audit events.
3. Moderator picks an action:
   - **Confirm** (NSFW only — body `{val: "porn" | "sexual" | "nudity"}`): the chosen canonical label is issued on every URI sharing the blob, the `currents-nsfw-suspected` label is negated on every URI, the item is marked `resolved`. For violence, the val is fixed to `graphic-media`.
   - **Take down**: `SetHarmState(blocked)`, `!hide` issued on every URI, suspected label negated, item `resolved`. Read queries immediately filter the blob.
   - **Dismiss**: suspected label negated on every URI, item `dismissed`. The auto-fired suspected label disappears for everyone.

All actions write a `moderation_event` row attributed to the moderator's DID.

---

## DB schema

Created by migrations [`016_moderation_core`](appview/migrations/016_moderation_core.up.sql) and [`017_moderation_workflow`](appview/migrations/017_moderation_workflow.up.sql).

| Table | Purpose | Key |
|---|---|---|
| `label` | Append-only log of every signed label issuance and negation. Source of truth for what's currently visible to clients (latest row per `(src, uri, val)` wins). | `id` BIGSERIAL |
| `blob_moderation_state` | Per-blob state (`harm_state`, `ai_generated` flag, `safety_scores`, decided_by/at, notes). Keyed by exact blob CID. | `blob_cid` PK |
| `moderation_event` | Append-only audit log of every action (`label_add`, `label_negate`, `takedown`, `acknowledge`, `ai_flag`, `self_label`). | `id` BIGSERIAL |
| `moderator` | Active moderator DIDs and their roles (`admin` / `reviewer`). | `did` PK |
| `report` | Raw user-submitted reports via `createReport`. | `id` BIGSERIAL |
| `review_item` | Pending items in the human queue. Sources: `ai`, `report`, `self_label`, `manual`. | `id` BIGSERIAL |

Partial indexes power the hot queries:

- `label_uri_active_idx ON label(uri) WHERE neg = FALSE` — read-time label hydration
- `label_blob_cid_active_idx ON label(blob_cid) WHERE neg = FALSE` — materialization onto new save URIs
- `blob_moderation_state_blocked_idx ON blob_moderation_state(blob_cid) WHERE harm_state = 'blocked'` — read-query filter
- `review_item_queue_idx ON review_item(status, priority DESC, created_at)` — queue listing

---

## Labeler protocol surface

The atproto labeler lives at `https://moderation.<your-host>` (a separate subdomain reverse-proxied to the appview). It's a module *inside* the appview binary (not a separate process), with its own DID, signing key, and DID document.

### DID document

Served at `https://moderation.<your-host>/.well-known/did.json` by [`appview/labeler_did.go`](appview/labeler_did.go) when the request's `Host` header matches the labeler subdomain:

```json
{
  "@context": [
    "https://www.w3.org/ns/did/v1",
    "https://w3id.org/security/multikey/v1"
  ],
  "id": "did:web:moderation.<your-host>",
  "verificationMethod": [{
    "id": "did:web:moderation.<your-host>#atproto_label",
    "type": "Multikey",
    "controller": "did:web:moderation.<your-host>",
    "publicKeyMultibase": "z..."
  }],
  "service": [{
    "id": "#atproto_labeler",
    "type": "AtprotoLabeler",
    "serviceEndpoint": "https://moderation.<your-host>"
  }]
}
```

### Label signing

Labels are signed `com.atproto.label.defs#label` records. Each row in the `label` table corresponds to one signed record. Signing uses indigo's `atproto/labeling` package: the canonical CBOR encoding of the label (excluding `sig`) is hashed and signed with secp256k1. See [`appview/labeler_signer.go`](appview/labeler_signer.go) and [`appview/labeler_issuer.go`](appview/labeler_issuer.go).

### Endpoints

| Endpoint | File | Notes |
|---|---|---|
| `GET /xrpc/com.atproto.label.queryLabels` | [`labeler_query.go`](appview/labeler_query.go) | `uriPatterns[]` required; `*` → SQL `LIKE` `%`. Cursor over `label.id`. |
| `GET /xrpc/com.atproto.label.subscribeLabels` | [`labeler_subscribe.go`](appview/labeler_subscribe.go) | WebSocket. Subscribe-before-backlog sequencing avoids racing labels issued during initial catch-up. CBOR frames per indigo's `events.EventHeader`. |
| `POST /xrpc/com.atproto.moderation.createReport` | [`labeler_report.go`](appview/labeler_report.go) | Strongly-typed `RepoStrongRef` subject only. Accepts both session cookie (first-party UI) and Bearer JWT (PDS-proxied). |

### Where do labels live?

The atproto [label spec](https://atproto.com/specs/label) mentions labels-in-records, which has confused readers. There are two distinct distribution mechanisms and they belong in different places:

| Mechanism | Storage | Who writes it | We do this |
|---|---|---|---|
| **Self-labels** — creator's voluntary declaration about their own content, attached to the labeled record itself under the `com.atproto.label.defs#selfLabels` shape | Inside the labeled record, in the **subject's own repo** on their PDS | The creator at record-write time | ✅ Upload picker (web + extension) → `parseSelfLabels` in [`appview/save_content.go`](appview/save_content.go) → embedded in the save record on the PDS. The TAP listener re-issues each as a **blob-keyed** labeler-signed copy so consumers see it without fetching the record, and so every copy of the same bytes (existing siblings + future resaves) inherits the warning. We never forge a self-label into a resaver's repo — only the original creator's repo holds their declaration; propagation is labeler-issued. |
| **Labeler-issued labels** — signed `com.atproto.label.defs#label` records produced by a labeler (auto-classified, reviewer-applied, etc.) | The **labeler's own database**, distributed via `subscribeLabels` / `queryLabels`. **NOT in any user's repo.** | The labeler (us). Third-party writes to a user's repo would violate the atproto trust model. | ✅ `label` table in Postgres + the XRPC endpoints above. Matches Ozone's design (Bluesky's reference labeler also keeps labels in its own DB, not in subject repos). |

So when the spec mentions labels associated with a record, what lives *in the record's own repo* is only the creator's self-labels. Labels you didn't ask for are in the labeler's database, served over the wire — same as how Bluesky's own moderation works.

---

## State machine

```
                    +-----+
                    |clean| <----------------------------+
                    +--+--+                              |
                       |                                 |
       safety score ≥ 0.97 (auto-flag)                   |
                       |                                 |
                       v                                 |
              (no harm_state change;                     |
               suspected label issued;                   |
               review_item enqueued)                     |
                       |                                 |
                       +-------+----------+              |
                               |          |              |
                       reviewer confirms  reviewer       |
                       (issue canonical;  dismisses      |
                        negate suspected) (negate susp.) |
                               |          |              |
                       (stays "clean";    +--------------+
                        canonical label is
                        the new signal)
                               |
                       +---taken down---+
                               |
                               v
                          +-------+
                          |blocked|---+
                          +-------+   |
                                      |
                              read queries filter;
                              !hide label issued on every URI
```

For v1, `harm_state` only has two effective values: `clean` and `blocked`. The `flagged` value is reserved but unused — the suspected-label rows are the de-facto pending state.

---

## Self-labels

Creators can voluntarily label their own content by attaching atproto's `com.atproto.label.defs#selfLabels` block to their save record (declared on `is.currents.feed.save`, defs in [`lexicons/com/atproto/label/defs.json`](lexicons/com/atproto/label/defs.json)). The web upload UI and the browser-extension clipper both expose a chip picker: the four Bluesky content warnings (`porn`, `sexual`, `nudity`, `graphic-media`, which blur) plus the Currents `currents-ai-generated` provenance label (which only badges — same val and rendering as the auto-classifier's AI flag, so self-disclosure and detection converge). Selected values are CSV-encoded in the `labels` form field. `CreateSave` (`appview/records.go`) parses with `parseSelfLabels` (`appview/save_content.go`), which validates against `allowedSelfLabelVals` and silently drops anything else (no arbitrary-label injection via the form).

A content warning is a property of the **bytes**, not of one record, so when the save lands via TAP, `applyModerationAfterSaveUpsert` issues a **blob-keyed** labeler-signed label per self-label, attributed to the author (`actor = save.AuthorDID`, `action = self_label`). Being blob-keyed, the warning propagates to every copy of the same bytes: existing copies via a retroactive fan-out (`propagateLabelToBlobSiblings`, once per `(blob, val)`), future copies via the standard materialization on the next blob-CID lookup. Self-labels only ever **blur** (never take down / `!hide`), are per-viewer-overridable, fully audited in `moderation_event`, and moderator-negatable — so the design biases toward propagating warnings (a wrongful blur is low-harm and reversible; a missed NSFW is not).

Crucially, we never forge a `labels` block into a resaver's PDS record: only the original creator's repo holds their own declaration. Propagation onto other copies is **labeler-issued** (the labeler making a claim, which the atproto trust model permits). Historical URI-scoped self-labels declared before this change are migrated by the `appview backfill-self-labels` CLI (`appview/backfill_self_labels.go`).

Creators can also add self-labels **after** upload (e.g. to warn on Pinterest-imported saves) via `PUT /save/{id}/labels` (`UpdateSaveLabels` in `appview/records.go`), surfaced as a picker on the save-detail page for owned saves — and **in bulk** from a collection's 3-dots menu (`PUT /save/labels/bulk` → `UpdateSaveLabelsBulk`, which fans the shared `applyLabelsToOwnedSave` helper out across the selected saves with bounded concurrency; the collection page enters a selection mode backed by `selectable-save-grid.svelte`). Both are **add-only** (merge with existing self-labels; removal is intentionally unsupported — adding a self-label is monotonic and can't weaken moderation) and **disallowed on resaves** (only originators declare; a non-resave is exactly as safe as upload-time labeling — resaves are skipped server-side in the bulk path regardless of the UI). It rewrites the record's `labels` field, then the TAP listener re-issues and fans out via the normal save-upsert path. Note: `UpdateSave` and `UpdateSaveAttribution` preserve the `labels` field on `RepoPutRecord` (which replaces the whole record), so editing a save's other fields no longer strips its self-labels.

---

## Backfill

`appview backfill-moderation` ([`backfill_moderation.go`](appview/backfill_moderation.go)) re-scores historical saves. It iterates blobs lacking a `blob_moderation_state` row, batches embeddings to `/classify/safety/embeddings`, and applies the same threshold ladder — but it issues labels on **every** URI sharing the blob (not just one source), since there's no future-resave materialization to rely on for already-indexed content.

Flags: `--batch-size` (default 512), `--interval` (default 1s, throttles to avoid starving live enrichment), `--limit N` (staged rollout), `--dry-run` (log decisions for one batch and exit without writes; safe without the labeler key).

---

## Code map

| Concern | File(s) |
|---|---|
| Migrations | `appview/migrations/016_moderation_core.{up,down}.sql`, `appview/migrations/017_moderation_workflow.{up,down}.sql` |
| Threshold constants + classifier helpers | `appview/moderation.go` |
| Store methods | `appview/pgstore.go` (search for `// ── Moderation methods`) |
| Live TAP integration | `appview/tap.go` (`handleSaveUpsert`, `processBlobEnrichment`) + `appview/moderation_tap.go` (`processSafetyScores`, `applyModerationAfterSaveUpsert`) |
| Labeler signing/issuing | `appview/labeler_signer.go`, `appview/labeler_issuer.go` |
| Labeler endpoints | `appview/labeler_did.go`, `appview/labeler_query.go`, `appview/labeler_subscribe.go`, `appview/labeler_report.go` |
| Admin backend | `appview/admin.go` (`requireModerator` middleware + queue handlers) |
| Self-label parsing + propagation | `appview/save_content.go` (`parseSelfLabels`, `buildSelfLabelsRecord`) + `appview/records.go` (`CreateSave`) + `appview/moderation_tap.go` (`applyModerationAfterSaveUpsert`, `propagateLabelToBlobSiblings`) |
| Self-label pickers | `frontend/src/routes/(with-navbar)/upload/+page.svelte` + `extension/src/entrypoints/clipper.content/App.svelte` (`labels` form field → `CreateSave`) |
| XRPC label hydration | `appview/save_content.go` (`hydrateLabels`, `labelView`, `Labels` field on `saveView`) + every save-listing handler in `appview/xrpc.go` |
| Inference heads | `inference/main.py` (`_load_safety_heads`, `_classify_safety`, `_l2_normalize`, `/classify/safety/embeddings`) |
| Backfill CLI | `appview/backfill_moderation.go` (re-score blobs), `appview/backfill_self_labels.go` (propagate historical self-labels) + subcommands in `appview/main.go` |
| Admin UI | `frontend/src/routes/(admin)/admin/*` |
| Label rendering wrapper | `frontend/src/lib/components/labeled-media.svelte` |
| Report dialog | `frontend/src/lib/components/report-dialog.svelte` |
| Self-label picker | `frontend/src/routes/(with-navbar)/upload/+page.svelte` |
| Viewer preferences | `frontend/src/lib/stores/moderation-prefs.svelte.ts` + `frontend/src/routes/(with-navbar)/settings/+page.svelte` |
| Type definitions | `frontend/src/lib/types.ts` (`LabelView`, `labels` on `SaveView`) |

---

## What's intentionally NOT here

These came up during design and were deferred — links to where they'd land later:

- **Account-level actions** (suspending an actor). Schema would extend `moderator_event` and add an `actor_state` table. Read queries would need a `WHERE NOT EXISTS (suspended_actor ...)` filter, similar to the blob-blocked filter.
- **CSAM pipeline.** Needs separate legal review, separate retention policy, dedicated hash-matching against IWF / NCMEC lists. Should never enter the normal `review_item` queue.
- **EU DSA transparency reporting.** Non-engineering.
- **Admin extras:** `/admin/reports`, `/admin/blob/[cid]`, `/admin/events`, `/admin/ai-generated`. Each needs a backend endpoint (`GET /api/admin/reports` etc.) plus a Svelte route. Backend pattern is in [`admin.go`](appview/admin.go); UI pattern is in [`(admin)/admin/queue`](frontend/src/routes/(admin)/admin/queue/).
- **Appeal flow** for creators disputing an auto-applied `currents-ai-generated` label — the plan is a "Not AI?" button on owned saves that creates a `manual` `review_item`.
- **One-shot CLI to publish `app.bsky.labeler.service`** — not built; use existing atproto tools (see [`MODERATION_DEPLOYMENT.md`](MODERATION_DEPLOYMENT.md)).
- **Per-head threshold tuning.** Uniform constants today; revisit when eval sets exist.
