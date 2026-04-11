# How Currents Ranks, Searches, and Personalizes

This document explains the ML and retrieval systems behind Currents — how images are embedded, how search works, how the discovery feed is ranked, and how the personalization slider blends the two.

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

The `getFeed` endpoint returns a discovery feed. Its behavior depends on the **personalization slider** (`personalized` parameter, 0.0–1.0).

### Global feed (personalization = 0)

A single query over all visual identities, ranked by a **time-decayed popularity score**:

```
score = save_count × exp(−0.01 × age_in_days)
```

- `save_count` rewards images that have been saved by multiple users.
- The exponential decay (same formula used elsewhere in the app, e.g. collection importance) means a 100-day-old image's score is ~37% of a brand-new image with the same save count. A 1-year-old image decays to ~2.5%.
- No minimum `save_count` — every image with a visual identity appears. Unpopular or new images simply rank lower.
- Pagination is a clean offset over this single sorted result set.

### Personalized feed (personalization > 0)

The slider linearly interpolates between personalized and global results:

- `personalizedLimit = round(limit × alpha)`
- `explorationLimit = limit − personalizedLimit`

**Personalized portion:**

1. The viewer's **top 3 collections** are selected, ranked by a time-decayed importance score (PinSage-inspired formula: `SUM(exp(−0.01 × age_days))` over saves in each collection). Only collections with a precomputed `canonical_embedding` qualify.
2. Each collection's `canonical_embedding` (the **medoid** of its saves' embeddings — the real embedding with minimum total cosine distance to all others, O(n²)) is used as a query vector.
3. For each collection, `perCol = max(1, personalizedLimit / numCollections)` results are fetched via the same pgvector ANN search used by text search, but with the collection embedding as the query instead of text.
4. Results are deduplicated across collections.

**Exploration portion:**

The remaining slots are filled by the global feed (same time-decayed popularity ranking described above), skipping any images already included in the personalized portion.

### Example at different slider values

| Slider | Personalized slots | Global slots | Behavior |
|--------|-------------------|--------------|----------|
| 0.0 | 0 | 50 | Pure global feed — popularity × recency |
| 0.3 | 15 | 35 | Mostly exploration, light personalization |
| 0.7 | 35 | 15 | Mostly "more like your collections" |
| 1.0 | 50 | 0 | Fully personalized — only ANN results from your top collections |

### Collection embedding updates

When a new save arrives in a collection, the collection's `canonical_embedding` is recomputed asynchronously (debounced, 30 s delay). The medoid is chosen because it is always a real image embedding (unlike a centroid, which may land in a sparse region of the space), making ANN lookups more reliable.
