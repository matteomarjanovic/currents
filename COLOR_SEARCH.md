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
only images whose palette contains a color within ΔE ≤ `colorHybridMaxDeltaE`
covering at least `colorHybridMinFraction` of the image, then order those by
cosine distance of the image embedding to the text-query embedding.

`SearchHybridSavesPage` in `appview/pgstore.go` is the existing semantic query
plus **one clause**:

```sql
... FROM visual_identity vi JOIN save s ON s.uri = vi.canonical_save_uri
WHERE vi.embedding IS NOT NULL
  AND <hidden / moderation / scope filters>
  AND EXISTS (SELECT 1 FROM visual_identity_color c
              WHERE c.visual_identity_id = vi.id
                AND c.lab <-> $lab <= $maxDeltaE
                AND c.fraction >= $minFraction)
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

## The hybrid gate (`colorHybridMinFraction` 0.05, `colorHybridMaxDeltaE` 20)

Both constants are **hybrid-only** and stricter than pure color search's ΔE 25.
They were tuned over three passes; the history matters because it shows which
dial does what and why the endpoints landed where they did.

**Why hybrid needs its own thresholds.** Pure color search ranks by
`ΔE − colorCoverageWeight·fraction`, so a weak match is *demoted*, not admitted
at the top; its ΔE 25 is the outer bound of a tail nobody scrolls to. Hybrid
orders by semantics alone, so the gate is its **entire** color criterion — a
2%-coverage accent at ΔE 24 is exactly as eligible for position 1 as an image
that's 60% the exact color. Same threshold, very different exposure. Hybrid can
afford stricter thresholds because the text query carries recall — but only up to
a point, which is what pass 3 found.

**The two dials are not interchangeable.** The coverage floor is *hue-neutral*:
raising it drops low-coverage matches regardless of color, and never
preferentially re-admits greys. ΔE is *hue-selective but chroma-blind*: it's the
only dial that excludes wrong hues, but because ΔE76 is plain L2 in Lab it can't
tell a muted target color from grey (see the limitation below). So: reach for the
floor to trade recall against accent-noise, reach for ΔE to trade recall against
hue-drift — and know that tightening ΔE punishes muted queries hardest.

### Pass 1 — add the coverage floor (0.08)

Complaint: matches were a *speck* of the color. Measured over the dev catalog
(39,577 indexed visual identities, July 2026), images whose best palette match
passes:

| query color | ΔE ≤ 25 | + `fraction ≥ 0.08` | ΔE ≤ 15 instead |
|---|---|---|---|
| mid blue `#3b6fb5` | 3,560 | 2,826 (−21%) | 1,190 (−67%) |
| mustard `#d9a441` | 3,506 | 2,588 (−26%) | 981 (−72%) |
| forest green | 871 | 697 (−20%) | 127 (−85%) |
| red `#ff0000` | 210 | 125 (−40%) | 56 (−73%) |

Coverage was the right *first* lever: a scalpel where ΔE was a hatchet that cut
hardest exactly where rare-color recall already hurts.

### Pass 2 — tighten ΔE 25 → 18

Complaint: matches were now well-covered but the *wrong hue*. The median survivor
sat at ΔE 18–21 and the largest single band was 20–25; sampling what a mid-blue
`#3b6fb5` query matched there — `#275677` `#536480` `#56647e` `#6b7390` — showed
uniformly **desaturated slate greys**. Big enough regions, wrong color. ΔE 18 cut
that band.

### Pass 3 — loosen to ΔE 20 / floor 0.05

Complaint: 18/0.08 was *too* tight — the color matched perfectly, but the color
pool was so small that hybrid's semantic ranking had too few on-topic candidates
to surface, so the top results drifted off the text query. Hybrid ranks semantics
*over the filtered pool*, so an over-tight gate starves the ranker. Loosening
restored the pool for the common (saturated) case without re-admitting greys:

| query | pool at 18/0.08 | pool at 20/0.05 | grey share at 20/0.05 |
|---|---|---|---|
| red | 53 | 92 | 0% |
| mid blue | 1,402 | 2,025 | 0% |
| mustard | 1,182 | 1,825 | 0% |
| green | 187 | 312 | 0% |
| teal (muted) | 2,101 | 4,494 | 63% |

For saturated queries the +40–75% recovered is entirely correct-hued. The floor
came down to 0.05 (hue-neutral recall) and ΔE up only to 20 (still 5 under pure
color search), because ΔE is the dial that trades against muted queries — which
is the standing limitation:

### Known limitation: neutrals contaminate low-chroma queries

ΔE76 is plain L2 in Lab, so a fully neutral grey sits ΔE ≈ *the query's chroma*
away from it. A muted query color therefore has the whole black/grey axis inside
its ball. Real example: query `#0c4740` (dark teal, chroma ≈ 20) matched a
near-white graphic on its `#2b2b29` near-black at 11% coverage, ΔE 21.7 — the
teal is simply not in that image.

| `#0c4740` | survivors | of which near-neutral (chroma < 10) |
|---|---|---|
| ΔE ≤ 25 | 15,883 (40% of the catalog) | 12,104 (76%) |
| ΔE ≤ 20 | 4,494 | 2,831 (63%) |
| ΔE ≤ 18 | 2,106 | 1,045 (50%) |

Even ΔE 18 leaves **half** the muted-query pool grey. A flat threshold tight
enough to exclude neutrals for a muted query (~12) would be far too tight for a
saturated one, so this is not fixable by moving the number — it caps how far ΔE
can loosen before muted queries fill with grey, which is why pass 3 leaned on the
floor instead. The
structural fix is a chroma guard (require the matched color's chroma to be a
minimum fraction of the query's) or a chroma-weighted distance / ΔE2000 —
neither is implemented; ΔE76 is what the HNSW index speaks.

### Not applied to `SearchSavesByColorPage`

Its ranking already demotes both weak-coverage and distant-hue matches, so
gating there would cut recall — on rare colors especially — and buy no ordering
improvement. `TestHybridColorGate` in `pgstore_db_test.go` pins the asymmetry:
hybrid drops both an accent-only match (even when it's the nearest semantic
neighbour) and a well-covered but visibly different hue, while pure color search
keeps all three and orders them best-match first.

The known cost is the anticipated one: colors that only ever appear as accents,
or that are rare in the catalog, return less. Loosen the floor before loosening
ΔE.
