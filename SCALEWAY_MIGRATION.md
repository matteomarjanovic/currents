# Migrating the appview stack to a Scaleway Virtual Instance

Moves `db + tap + appview + clustering + nginx + cloudflared` from the mac
mini to a Scaleway Virtual Instance. **Only inference stays on the mac
mini** (it targets Apple Silicon / MPS); the appview reaches it over
Tailscale. Clustering runs next to the DB and hands its monthly UMAP model
to inference through a **model store** (Object Storage bucket): clustering
uploads after training, the mini syncs the bucket on a cron. Postgres
publishes no port anywhere.

Why this shape and not managed PG / Dedibox / one-VM-per-service: see
`SCALING.md` (phases) and the cost analysis in `BACKLOG.md`.

End state:

```
Scaleway VM (docker-compose.scaleway.yml)         mac mini (tailnet)
┌─────────────────────────────────────┐           ┌─────────────────────┐
│ cloudflared → nginx → appview ──────┼──────────▶│ inference (uvicorn) │
│                        │    ▲       │  :8000    │        ▲            │
│ tap ──────────────▶ Postgres        │           │  s3 sync + reload   │
│  ▲                  /mnt/pgdata     │           │  (daily cron)       │
│  └── relay1.us-east     ▲           │           └─────────▲───────────┘
│                     clustering ─────┼───┐                 │
└─────────────────────────────────────┘   └──▶ Object Storage: currents-models
```

## 1. Provision (Scaleway console)

- **Instance**: BASIC2-A2C-8G (2 vCPU / 8 GB) — or PRO2-XXS for dedicated
  cores. Ubuntu 24.04 LTS. Paris region (closest to users; the relay hop is
  from the VM either way).
- **Block Storage volume**: 30 GB (5K IOPS class), attached to the instance.
  This holds `pgdata`; it's what survives instance resizes and rebuilds.
- **Security group**: inbound **SSH only** (ideally restricted to your IPs).
  Nothing else — cloudflared and Tailscale are outbound-only.
- **Object Storage buckets** (both in `fr-par`), plus one API key scoped to
  the pair — used by the VM (backup upload, model publish) and the mac mini
  (model sync):
  - `currents-db-backups` — nightly dumps; lifecycle rule expiring objects
    after 30 days.
  - `currents-models` — the model store; **enable bucket versioning** (that's
    the rollback story for a bad UMAP retrain).

## 2. Prepare the VM

```bash
# Docker
curl -fsSL https://get.docker.com | sh

# Tailscale — joins the same tailnet as the mac mini (outbound only: the VM
# needs it to reach inference; nothing dials in)
curl -fsSL https://tailscale.com/install.sh | sh
tailscale up

# Format and mount the block volume (find the device with lsblk — e.g. /dev/sda)
mkfs.ext4 /dev/sda
mkdir -p /mnt/pgdata
echo '/dev/sda /mnt/pgdata ext4 defaults,nofail 0 2' >> /etc/fstab
mount -a

# The repo
git clone https://github.com/<you>/currents.git /opt/currents
```

Copy the mac mini's `.env` to `/opt/currents/.env` and add/adjust:

```
INFERENCE_URL=http://100.a.b.c:8000       # the mac mini's tailscale IP
MODELS_S3_BUCKET=currents-models
S3_ACCESS_KEY=SCW...                      # the bucket-scoped API key
S3_SECRET_KEY=...
```

Everything else (`DOMAIN`, `SESSION_SECRET`, `CLIENT_SECRET_KEY`,
`TAP_ADMIN_PASSWORD`, `POLAR_*`, `CLOUDFLARE_TUNNEL_TOKEN`, …) carries over
unchanged. `DB_PASSWORD` is now required (no `appview` fallback) — if the
mini ran with the default, set a real one and it takes effect on restore.

Note on containers reaching the tailnet: outbound traffic from bridge
containers to `100.x` addresses routes through the host's `tailscale0`
automatically — no sidecar or host networking needed.

## 3. Migrate the data

One dump/restore carries everything — appview tables **and** TAP's tables
(`repos`, `repo_records`, `firehose_cursors`, outbox). Because the firehose
cursor travels with the data, TAP on the VM resumes from where the mini left
off and replays the gap from the relay — nothing is lost as long as the
cutover doesn't take longer than the relay's replay window (days).

```bash
# On the mac mini — stop writers, keep db up:
docker compose -f docker-compose.mac-mini.yml stop appview tap clustering
docker exec currents-db-1 pg_dump -U appview -d appview -Fc > currents.dump

# Ship it (over the tailnet):
scp currents.dump root@<vm-tailscale-ip>:/opt/currents/

# On the VM — restore into a fresh db, then start everything:
cd /opt/currents
docker compose -f docker-compose.scaleway.yml up -d db
docker compose -f docker-compose.scaleway.yml exec -T db \
  pg_restore -U appview -d appview --no-owner < currents.dump

# Seed the models directory (clustering needs the current UMAP model for its
# nightly backfill until the next monthly retrain) and publish it to the
# bucket while at it:
scp <mini>:~/currents/models/umap_model.joblib /opt/currents/models/
aws --endpoint-url https://s3.fr-par.scw.cloud \
  s3 cp /opt/currents/models/umap_model.joblib s3://currents-models/

docker compose -f docker-compose.scaleway.yml up -d --build appview tap clustering nginx
```

Sanity-check the restore before cutting over:

```bash
docker compose -f docker-compose.scaleway.yml exec db \
  psql -U appview -d appview -c "SELECT count(*) FROM save;" \
                             -c "SELECT count(*) FROM visual_identity;" \
                             -c "SELECT * FROM firehose_cursors;"
```

## 4. Cut over the tunnel

The Cloudflare tunnel token identifies the tunnel, not the machine — run it
from the VM instead of the mini. **Never both at once.**

```bash
# mac mini:
docker compose -f docker-compose.mac-mini.yml stop cloudflared
# VM:
docker compose -f docker-compose.scaleway.yml up -d cloudflared
```

Then verify end to end: log in on the web client, save an image, and watch
it arrive (`docker compose logs -f appview` → "TAP save received"), check
search and the feed, and confirm the TAP cursor advances
(`SELECT * FROM firehose_cursors` twice, a minute apart).

## 5. Re-point the mac mini

The mini now runs **only inference** (uvicorn, as today). It picks up new
UMAP models from the model store with a daily sync — the inference server's
`/reload-umap` is mtime-based and a no-op when nothing changed, so blind
daily syncing is correct and cheap (the bucket changes once a month):

```bash
# /Users/matteo/currents-model-sync.sh
#!/bin/sh -e
export AWS_ACCESS_KEY_ID=SCW... AWS_SECRET_ACCESS_KEY=...
aws --endpoint-url https://s3.fr-par.scw.cloud \
  s3 sync s3://currents-models /Users/matteo/projects/currents/models
curl -fsS -X POST http://localhost:8000/reload-umap
```

`crontab -e` on the mini → `0 6 * * * /Users/matteo/currents-model-sync.sh`
(06:00 — after the monthly train job's 02:00 slot has finished).

```bash
# Then stop the old stack for good:
docker compose -f docker-compose.mac-mini.yml down
```

Keep the old `pgdata` docker volume on the mini untouched for a couple of
weeks — it's the instant rollback (stop VM stack, `up` the mini stack,
restore the latest dump if the VM ran for long).

## 6. Backups

Three layers, cheapest first:

1. **Nightly `pg_dump` to Object Storage.** On the VM:

   ```bash
   # /opt/currents/backup.sh
   #!/bin/sh -e
   f=/tmp/currents-$(date +%F).dump
   docker compose -f /opt/currents/docker-compose.scaleway.yml exec -T db \
     pg_dump -U appview -d appview -Fc > "$f"
   aws --endpoint-url https://s3.fr-par.scw.cloud \
     s3 cp "$f" s3://currents-db-backups/
   rm "$f"
   ```

   `apt install awscli`, configure the bucket-scoped key, then
   `crontab -e` → `10 4 * * * /opt/currents/backup.sh`. The bucket's 30-day
   lifecycle rule handles retention.

2. **Weekly Block Storage snapshot** of the pgdata volume (Scaleway console
   or `scw block snapshot create` from a cron; €0.03/GB/month).

3. **The network itself.** Most of the DB is derived state: saves,
   collections, follows and profiles rebuild from PDSes via TAP backfill;
   embeddings, palettes and junk scores rebuild via the `appview backfill-*`
   commands; `polar_subscription` re-syncs from Polar. The irreplaceable
   tables are the small ones (moderation state and review history,
   `seen_feature`, `moderation_pref`, `starred_collection`, `color_trial`) —
   layer 1 covers them many times over.

## Ops notes

- **Resize** (more RAM): stop instance → change type in console → start.
  pgdata is on the block volume, so this is minutes of downtime and no data
  movement. Raise `shared_buffers` in the compose to ~25% of the new RAM.
- **Rebuild from scratch**: new instance, attach the volume (or restore a
  snapshot into a new volume), redo step 2, `up -d`. The tunnel token and
  `.env` are the only secrets to bring.
- **TAP fell behind / repo out of sync**: same recipe as on the mini —
  `UPDATE repos SET state='desynchronized', retry_count=0, retry_after=0
  WHERE did='…';` and the resyncer re-fetches the repo and diff-emits.
- **Bad UMAP retrain**: restore the previous object version of
  `umap_model.joblib` in the `currents-models` bucket (versioning is on),
  re-run the mini's sync script, and re-run `train_umap.py`'s re-projection
  when a good model lands. A failed *upload* is loud by design: the train
  cron exits non-zero rather than letting the mini sync a stale model
  silently.
- **Inference down** (mini offline): saves index with
  `visual_identity_id IS NULL` and stay invisible to paid search until the
  repair pass runs — that's the existing `APPVIEW_MODE=repair` routine from
  `BACKLOG.md`, unchanged by this migration.
