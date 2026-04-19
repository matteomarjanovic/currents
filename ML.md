# How Currents Ranks, Searches, and Personalizes

This document explains the ML and retrieval systems behind Currents — how images are embedded, how search works, how the discovery feed is ranked, and how the feed personalization slider changes retrieval behavior.

## Embedding model

All embeddings come from **SigLIP2** (`google/siglip2-base-patch16-naflex`), a multimodal vision–language model that maps images and text into a **shared 768-dimensional vector space**. The inference server (`inference/main.py`) exposes two endpoints:

- `/embed/image` — encodes a single image (direct execution, no batching).
- `/embed/text` — encodes text queries (queue-based micro-batching, up to 32 texts per batch, 20 ms max wait).

Because images and text share the same embedding space, text-to-image retrieval is a simple nearest-neighbor lookup — no separate cross-encoder or re-ranker is needed.

## Visual identity

Every unique image in the system is represented by a **visual identity** row (`visual_identity` table). When a save is indexed via the TAP listener:

1. If the image blob CID already exists → reuse the existing visual identity.
2. Otherwise → fetch the blob, call `/embed/image`, and create a new visual identity with the embedding, dominant-color palette, and a `save_count` of 1.

The `save_count` field tracks how many saves reference this visual identity (maintained by a DB trigger). The `canonical_save_uri` points to the representative save for this image — the one surfaced in feed and search results.

Embeddings are stored as `VECTOR(768)` columns in PostgreSQL via **pgvector**, indexed with **HNSW** (`vector_cosine_ops`) for fast approximate nearest-neighbor search.

## Search

The `searchSaves` endpoint performs **text-to-image retrieval**:

1. The query text is embedded via `/embed/text` → 768-d vector.
2. pgvector runs ANN search over `visual_identity.embedding` using cosine distance (`<=>`).
3. Results are returned ordered by cosine distance (most similar first).

There is no minimum popularity threshold — every image with an embedding is searchable. Pagination is offset-based.

## Discovery feed

The `getFeed` endpoint returns the discovery feed. Its behavior depends on the signed **personalization slider** (`personalized` parameter, `-1.0` to `1.0`):

- `0` = pure global feed.
- Positive values = personalized feed based on the viewer's own collection medoids.
- Negative values = serendipity feed based on nearby cluster medoids that the viewer has not touched.

The feed is implemented as a small set of candidate pools, mixed together with weights derived from `abs(personalized)`. Search does not currently use this signed feed behavior.

### Global pool (`personalized = 0`)

The global pool is a single query over all visual identities, ranked by a **time-decayed popularity score**:

```
score = save_count × exp(−0.01 × age_in_days)
```

- `save_count` rewards images that have been saved by multiple users.
- The exponential decay (same formula used elsewhere in the app, e.g. collection importance) means a 100-day-old image's score is ~37% of a brand-new image with the same save count. A 1-year-old image decays to ~2.5%.
- No minimum `save_count` — every image with a visual identity appears. Unpopular or new images simply rank lower.
- When the feed is purely global, pagination is a clean offset over this single sorted result set.

### Positive personalization (`personalized > 0`)

Positive personalization builds up to 3 personalized pools from the viewer's own collections:

1. The viewer's **top 3 collections** are selected, ranked by a time-decayed importance score:

	```
	importance = SUM(exp(−0.01 × age_in_days))
	```

	This is computed over the viewer's saves in each collection. Only collections with a precomputed `canonical_embedding` are eligible.

2. Each selected collection contributes its `canonical_embedding`, which is the **medoid** of that collection's saves' embeddings: the real embedding with minimum total cosine distance to all others.

3. Each collection medoid becomes one ANN retrieval pool by querying `visual_identity.embedding` with the same pgvector cosine-distance search used by text search.

4. The cursor stores the selected collection URIs plus per-pool offsets, so later pages continue paging the same collection-derived pools instead of recomputing the top-3 set on every request.

### Negative personalization / serendipity (`personalized < 0`)

Negative personalization starts from the same top 3 viewer collections, but swaps in **nearby unexplored cluster medoids** before retrieval:

1. The viewer's top 3 collections are selected exactly as in positive personalization.

2. Daily clustering assigns each visual identity to a cluster and stores a **cluster medoid** (a representative visual identity for that cluster).

3. For each of the viewer's top collections, the feed finds the **nearest cluster medoid** to that collection's `canonical_embedding`, subject to two exclusions:

	- Exclude clusters that already contain any of the viewer's saves.
	- Exclude clusters already selected by an earlier collection in the same request, so the 3 negative pools stay distinct.

4. Each selected cluster medoid contributes its own ANN retrieval pool, again querying the main 768-d `visual_identity.embedding` space. The cluster structure is only used to choose seeds; actual retrieval still happens in the shared SigLIP2 embedding space.

5. The cursor stores the selected cluster-medoid visual-identity IDs plus per-pool offsets, so later pages continue paging the same serendipity pools even though cluster IDs themselves are not stable pagination keys.

### Mixing global and personalized pools

The feed does not pre-allocate a fixed number of "personalized slots" and "global slots". Instead, it mixes candidate pools by weight.

Let:

```text
w = abs(personalized)
```

Then:

- Total personalized weight = `w`
- Each personalized pool gets weight `w / numPersonalizedPools`
- Global pool weight = `1 - w`

This means:

- `personalized = 0` -> pure global feed
- `personalized = 0.6` -> 60% positive-personalized weight, 40% global weight
- `personalized = -0.6` -> 60% serendipity weight, 40% global weight
- `personalized = 1` or `-1` -> strict personalized/serendipity mode, with no global fallback

If there are no personalized pools available and `abs(personalized) < 1`, the feed falls back to a pure global pool.

### Feed assembly and pagination

For each pool, the server prefetches a deeper ANN page than the outward-facing page size:

```text
fetchLimit = max(limit × 3, limit + 25)
```

This gives the mixer enough headroom to skip duplicates and still fill the page.

Feed assembly then works in two phases:

1. Repeatedly choose among pools with remaining items according to their weights.
2. Consume the next unseen item from the chosen pool.
3. If weighted selection exhausts early, fill the remaining slots in pool order.

Duplicates are removed within a page, and each pool's cursor offset advances by the number of candidate items actually consumed, not just by the number of results returned to the user.

Pagination is mode-aware:

- Global mode stores only a single global offset.
- Positive mode stores collection URIs plus per-collection offsets.
- Negative mode stores cluster-medoid visual-identity IDs plus per-seed offsets.

Because the cursor keeps the chosen pools fixed, page 2 and later continue the same retrieval plan instead of recomputing it.

### Example at different slider values

| Slider | Mode | Behavior |
|--------|------|----------|
| -1.0 | Strict serendipity | Only cluster-seeded ANN pools from nearby unexplored regions |
| -0.5 | Mixed serendipity | Half cluster-seeded serendipity, half global |
| 0.0 | Global | Pure popularity × recency |
| 0.5 | Mixed personalization | Half collection-seeded personalization, half global |
| 1.0 | Strict personalization | Only ANN pools seeded by the viewer's top collections |

### Collection embedding updates

When a new save arrives in a collection, the collection's `canonical_embedding` is recomputed asynchronously (debounced, 30 s delay). The medoid is chosen because it is always a real image embedding (unlike a centroid, which may land in a sparse region of the space), making ANN lookups more reliable.
