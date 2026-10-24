# Scaling plan — what to do as the library grows

The cost driver is **unique images (visual identities), not users**: blobs
live on users' PDSes (we never store images), and the dedup ratio only
improves as the network grows (more resaves of the same content). What we
pay for is vectors + metadata, and the binding constraint is **RAM for the
ANN indexes** — pgvector's HNSW keeps full-precision vectors inside the
index, and it has to be mostly cache-resident to be fast.

Baseline measured 2026-08-04 (≈400 users, 246k visual identities, 336k
saves, 3.6 GB database):

| Hot object | Size | Rule of thumb |
|---|---|---|
| `idx_vi_embedding` (HNSW fp32 768-d) | 962 MB | ~4 KB per visual identity |
| `idx_vic_lab` (HNSW color) | 247 MB | ~1 KB per VI |
| `idx_vi_umap_embedding` | 111 MB | clustering only |
| btrees (`save`, `collection`, …) | ~400 MB | |
| whole database on disk | 3.6 GB | ~15 KB per VI |

Each phase below has a **trigger** — don't do the work early. Check where
you stand with:

```sql
-- hot-set inventory
SELECT indexrelname, pg_size_pretty(pg_relation_size(indexrelid))
FROM pg_stat_user_indexes ORDER BY pg_relation_size(indexrelid) DESC LIMIT 8;

-- cache health (should stay > 0.99; sustained lower means the hot set
-- no longer fits and queries are hitting disk)
SELECT sum(blks_hit)::float / nullif(sum(blks_hit) + sum(blks_read), 0)
FROM pg_stat_database WHERE datname = 'appview';
```

## Backup and recovery scaling

Measured 2026-10-24: 916 indexed users, 1.27M saves, 869k visual identities,
4.30M palette rows, 1.81M import items, and about 12 GB of database storage.
`visual_identity` alone holds about 4.1 GB of data/TOAST and 4.1 GB of indexes;
`import_item` holds about 653 MB of data plus 288 MB of indexes. Account count
does not predict this workload: imports and unique images drive the stored
data, and deduplicated resaves have a different cost from new images.

The first full zstd level-1 backup on that date completed in 11m22s including
upload and validation, versus 16m55s for the preceding gzip backup; its archive
was about 3.5 GB. This is a live-workload comparison, not a throughput guarantee.
The nightly schedule and fresh backup before every appview deployment remain
unchanged.

Current backups export every row with `pg_dump`, serializing vector values and
compressing the result anew. Index definitions are included, but index contents
must be rebuilt during a logical restore. Faster compression helps the export;
index quantization helps memory and disk usage, but does not shrink stored
full-precision embeddings or eliminate the export/restore work.

Logical dumps remain useful for portability and selective recovery. As primary
disaster recovery at larger data volumes, use physical base backups plus
continuous WAL archiving, with incremental/differential backups where useful.
This avoids repeatedly serializing the whole logical dataset and permits
recovery to a chosen time within the retained, complete backup/WAL history.
Physical backups include indexes, trading a larger full baseline for avoiding
index rebuilds on restore. Changed files, WAL volume, storage throughput, and
restore/replay time still need measurement; PITR is not an instant restore or
zero-data-loss guarantee, and a replica alone does not protect against bad
writes or deletions.

Evaluate pgBackRest against the existing Docker, ARM64, and Scaleway Object
Storage setup. Set explicit loss and downtime budgets, measure backup/restore
throughput, monitor archive failures and retained WAL growth, and test isolated
recovery before retiring the current policy. This work is tracked in
`BACKLOG.md`; nightly and per-appview-deployment logical backups remain enabled.

References: [PostgreSQL backup guidance](https://www.postgresql.org/docs/18/app-pgdump.html),
[continuous archiving and PITR](https://www.postgresql.org/docs/18/continuous-archiving.html),
and [pgBackRest](https://pgbackrest.org/user-guide.html).

## Phase 0 — now (≤ ~1M visual identities): do nothing

The 8 GB instance holds a ~1.5 GB hot set with 2–3× headroom. Housekeeping
that keeps it that way:

- Prune `import_item` rows for long-completed jobs (286 MB of bookkeeping
  today, grows with every Pinterest import).
- Check whether `idx_vi_umap_embedding` (111 MB) is queried outside
  clustering runs; if not, it can be dropped and rebuilt by the clustering
  job when needed.
- Don't loosen the `FindNearestVI` dedup threshold (0.02) to save space —
  false merges are irreversible, collapse genuinely different images into
  one search result, and buy at best ~15% index size. It's a quality knob,
  not a cost knob. Natural resave-dedup at scale does the real work for
  free.

## Phase 1 — halfvec index (trigger: vector indexes > ~50% of RAM, or cache
hit ratio degrading)

pgvector (we run 0.8.x) supports fp16 vectors in the index. An expression
index halves `idx_vi_embedding` with essentially no recall loss:

```sql
CREATE INDEX CONCURRENTLY idx_vi_embedding_half ON visual_identity
  USING hnsw ((embedding::halfvec(768)) halfvec_cosine_ops);
DROP INDEX idx_vi_embedding;
```

Queries must cast the same way (`embedding::halfvec(768) <=>
$1::halfvec(768)`) — a mechanical change in `pgstore.go`'s ANN queries.
Same trick applies to `idx_vic_lab` if it ever matters (it's 3-d, it
won't). One reindex, ~2× headroom, no architecture change.

## Phase 2 — binary quantization + rerank (trigger: the halfvec index
approaches RAM again; roughly 2–4M VIs on 8 GB)

The canonical pgvector scale-out: HNSW over 1-bit-per-dimension vectors
(96 bytes each, ~30× smaller core), query by Hamming distance with
oversampling, rerank the candidates exactly:

```sql
CREATE INDEX ON visual_identity
  USING hnsw ((binary_quantize(embedding)::bit(768)) bit_hamming_ops);
```

```sql
-- two-stage: wide ANN pass on bits, exact cosine on the shortlist
SELECT * FROM (
  SELECT id, embedding FROM visual_identity
  ORDER BY binary_quantize(embedding)::bit(768) <~> binary_quantize($1)
  LIMIT 200                       -- ~4× the page size
) shortlist
ORDER BY embedding <=> $1 LIMIT 50;
```

This slots into `searchSaves` / `getRelatedSaves` / the feed's ANN pools
without touching ranking semantics (`ML.md`) or the color hard-filter
design (`COLOR_SEARCH.md` — the filter join is unchanged; only candidate
generation gets a cheaper first stage). Expect ~10× headroom on the same
hardware. Validate recall on real queries before deleting the old index —
SigLIP2 embeddings quantize well, but measure, don't assume.

A further option in this phase if needed: PCA to ~256-d (applied to **both**
image and text sides so the shared space survives), ~3× on top. Needs an
offline recall eval; more invasive because the stored vectors change.

## Phase 3 — grow or split the hardware (trigger: Postgres alone wants more
RAM than a cheap instance offers, or TAP resync storms visibly hurt query
latency)

In order of preference:

1. **Resize the instance** (pgdata is on Block Storage; minutes of
   downtime). 16 GB instances are ~€50/mo — fine.
2. **Split into two VMs**: Postgres on a RAM-heavy instance, `tap + appview
   + nginx + cloudflared` on a tiny one (they idle at ~600 MB combined),
   joined by a free private network. It's a `DATABASE_URL` change; nothing
   assumes colocation.
3. **Managed PG fallback** if self-managing stops being fun — same
   ecosystem, dump/restore back in.

## Phase 4 — bare metal (trigger: ~10M+ VIs, or when instance RAM pricing
crosses Dedibox; roughly the "100k users" mark)

Dedibox dedicated servers (the Online.net side of Scaleway) are the €/GB-RAM
arbitrage: 64 GB ECC + NVMe boxes for ~€50–110/mo vs ~€7/GB/mo for managed
RAM. Postgres moves there (same docker-compose, same dump/restore path);
the light services can stay on a small instance or move along.

What you give up: snapshots, block storage, instant resize — hardware
failure becomes a support ticket plus a restore. Mitigate with the same
nightly dumps plus, at that scale, a streaming replica on a small instance.

If even 64 GB stops being enough, the next move is **pgvectorscale /
StreamingDiskANN** (ANN designed to run from NVMe with quantized vectors in
RAM) — it keeps everything in Postgres. Prefer that over an external vector
store: search depends on in-database joins (color hard filter, junk gate,
viewer hydration, moderation exclusions) that a separate ANN service would
force us to reimplement.

## Rough cost curve (2026 prices, with the phases applied)

| Scale | VIs (est.) | Setup | ~€/month |
|---|---|---|---|
| now | 250k | 8 GB instance, fp32 HNSW | 28–37 |
| ~10k users | 3–6M | same instance, Phase 1–2 applied | 30–60 |
| ~100k users | ~30M | 16 GB instance **or** Dedibox 64 GB, Phase 2(+PCA) | 60–150 |

Revenue note: the supporter tier scales with the same user count that
drives these costs, and the paid features (library search, find-similar,
color search) are exactly the ANN surfaces the phases keep cheap.
