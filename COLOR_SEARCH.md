# Color search & hybrid text+color search

How Currents lets users find images **by color**, and by **text + color together**.
Both are supporter-gated ($7/mo tier) and run entirely on PostgreSQL + pgvector —
no separate vector DB.

## The palette (ingest side)

Every image's dominant-color palette is extracted by the inference server
(`inference/main.py`, `_dominant_colors`) at index time:

- Downscale to a 128×128 thumbnail with **nearest-neighbour** resampling (keeps
  small accent colors intact instead of blending them away).
- Convert to **CIELab** (`_srgb_to_lab`), over-cluster with k-means (k≈12), merge
  clusters closer than ΔE 10, then pick the final 5 by a score that weights
  **coverage × chroma** — so a small but saturated accent (the classic "tiny
  yellow bird") survives instead of a fourth shade of the background.
- Output stays a backward-compatible `[{hex, fraction}]` array.

The appview stores one row per palette color in **`visual_identity_color`**
(`migration 036`): `visual_identity_id`, `rank`, `lab VECTOR(3)`, `fraction`
(pixel coverage). It's keyed to the **visual identity** (the dedup layer), like
the SigLIP2 embedding — so near-duplicate saves share one palette. HNSW index
`idx_vic_lab` uses `vector_l2_ops`; **L2 distance in Lab ≈ ΔE76**, a perceptual
color difference. Hex→Lab conversion lives once in Go (`hexToLab` in
`appview/visual.go`), used for both indexing and queries, so the inference
contract stays hex-only.

Backfill for existing images: `appview backfill-colors` (re-extracts palettes
through the inference `/palette` endpoint and refreshes `save.dominant_colors`).

## Color search (`searchSavesByColor`)

`SearchSavesByColorPage` in `appview/pgstore.go`:

1. ANN candidate scan over `visual_identity_color` (top `colorCandidateLimit`
   = 600 rows by `lab <-> query`).
2. `DISTINCT ON (visual_identity_id)` — one row per image, keeping its best
   palette match.
3. Rank by `score = ΔE − colorCoverageWeight·fraction` (a dominant near-match
   outranks an accent-only exact match, à la Cosmos's coverage re-rank), with a
   `colorMaxDeltaE = 25` cutoff so a sparse catalog doesn't return oranges for
   pink.

Global scope joins the canonical save network-wide; library scope (`library=true`
/ `collections`) joins the viewer's own saves. Standard moderation/hidden filters,
offset pagination.

## Hybrid text + color search

**The design decision that matters:** *color is a hard filter, semantic
relevance is the sole ranking.* We do **not** blend a cosine distance and a ΔE
into one score.

This follows the [Cosmos case study](https://qdrant.tech/blog/case-study-cosmos/),
which found that fusing the two signals (reciprocal-rank-fusion / weighted score
blending) "blends relevance and hue too much" — a cosine distance and a ΔE aren't
comparable scales, and users treat a color they picked as a **requirement**, not
a nudge. Filter-first is also the standard recommendation for heterogeneous
signals where one is an approximate semantic match and the other a near-exact
attribute.

**Query contract:** "images *about* `<text>` that *feature* `<color>`" — keep
only images whose palette contains a color within ΔE ≤ threshold (**any
presence**, no coverage floor), then order those by cosine distance of the image
embedding to the text-query embedding.

`SearchHybridSavesPage` in `appview/pgstore.go` is the existing semantic query
plus **one clause**:

```sql
... FROM visual_identity vi JOIN save s ON s.uri = vi.canonical_save_uri
WHERE vi.embedding IS NOT NULL
  AND <hidden / moderation / scope filters>
  AND EXISTS (SELECT 1 FROM visual_identity_color c
              WHERE c.visual_identity_id = vi.id AND c.lab <-> $lab <= $maxDeltaE)
ORDER BY vi.embedding <=> $textVec
LIMIT $limit+1 OFFSET $offset
```

The color filter is a cheap correlated `EXISTS` over the image's ≤5 palette rows
(reached by the `visual_identity_color` primary key) — no second HNSW scan, and
the two distance scales never meet. Semantic order means **offset pagination
stays correct and stable**.

**pgvector planner note (important):** a selective `EXISTS` tips PostgreSQL's cost
estimate toward a seq scan + full sort, abandoning the embedding HNSW index. So
the hybrid query runs with `enable_seqscan = off` (set per-query via the
`forceIndexScan` flag on `queryANNSavePage` / `setANNQueryOptions`) to force the
`idx_vi_embedding` index scan; every other table in the query is reached by its
own index anyway, so there's no collateral damage. pgvector 0.8's
`iterative_scan = strict_order` then keeps scanning the index (up to
`max_scan_tuples`) until it has a full page of color-matching images.

- Endpoint: **`searchSavesByColor` with an optional `q`** (both scopes, gated). No
  `q` → pure color; with `q` → hybrid. Handler embeds `q` via `EmbedText` and
  dispatches to `SearchHybridSavesPage`.
- `hybridScanFloor` (500) raises the ef_search / scan depth for the hybrid path,
  since a rare color post-filters the semantic scan heavily.

**Known trade-off — rare-color recall:** if a color appears in very few images,
the semantic scan filters most candidates and may under-return within
`max_scan_tuples`. This is inherent to filter-first ANN, not a bug. Mitigations:
the raised scan floor, and honest thin/empty states ("No 'x' images in this
color — try a broader color or drop the text").

## Frontend

- **Explore** (`/search/color/<hex>` route, `search-command.svelte`): the color
  toggle keeps the text field enabled. Color only → color search; color + text →
  hybrid, carried as `/search/color/<hex>?q=<text>`.
- **Library / organize**: color search lives in `?color=<hex>`; text is ephemeral
  state; the two can coexist as hybrid. The canvas adds `q` to the
  `searchSavesByColor` call when both are set; the header shows a combined chip
  (swatch + text + collection filter). A palette swatch in the image detail opens
  a menu: copy the hex, or search that color in explore / in the library.

## Tightening the filter later

Today the hybrid (and pure-color) filter matches **any** palette presence within
ΔE — a color that's a tiny accent still qualifies. If "features this color" ever
feels too loose, require the matched color to be a meaningful share of the image
by adding **one line** to the `EXISTS` in `SearchHybridSavesPage` (and the
`WHERE de <= $7` block in `SearchSavesByColorPage`):

```sql
AND c.fraction >= 0.08   -- only count the color when it covers ≥ ~8% of the image
```

Expose it as a tunable constant (e.g. `colorHybridMinFraction`) next to
`colorMaxDeltaE`. The cost is more empty results for colors that only ever appear
as accents, which is why it's off by default.
