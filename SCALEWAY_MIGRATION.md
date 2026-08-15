# Migrating Currents to Scaleway

Moves the public Currents stack off Netlify and the mac mini onto Scaleway,
while keeping inference isolated on a small CPU instance that can be resized
independently.

This is deliberately a **two-cutover migration**:

1. Move `db + tap + appview + clustering` and inference without changing the
   frontend or OAuth identity.
2. Move the frontend to SvelteKit SSR and make `currents.is` the OAuth client
   identity.

Separating the cutovers keeps the existing Netlify SPA usable while the data
and TAP cursor move, and makes each rollback small.

## Settled decisions

- **Main instance:** `BASIC2-A2C-8G` (2 vCPU / 8 GB), running Docker Compose:
  Postgres, TAP, appview, clustering, SvelteKit Node, and Caddy.
  The measured hot DB set is still well inside 8 GB; see `SCALING.md`.
- **Inference instance:** start with `BASIC2-A2C-4G` (2 vCPU / 4 GB), running
  one CPU-only inference container. Two GB may boot the model but does not
  leave safe headroom for the 340 MB UMAP model, decoded images, model loading,
  Docker, and the OS. Four GB is the practical minimum; resize only if the
  production benchmark fails.
- **Network:** both instances live on one Scaleway Private Network in `fr-par`.
  The main instance has one stable Scaleway Flexible IPv4; only Caddy ports 80
  and 443 are public. Inference port 8000 is reachable only over the Private
  Network and is never public.
- **Public edge and DNS:** Cloudflare remains the authoritative DNS provider so
  development environments can keep using Cloudflare Tunnel. Production
  records are DNS-only and point directly at the main instance's Flexible
  IPv4; neither Cloudflare Tunnel nor the Cloudflare proxy is in the production
  HTTP path. Caddy terminates public HTTPS and manages ACME certificates. The
  authoritative DNS provider can be moved later without changing this
  application topology.
- **Web deployment:** the web build uses `@sveltejs/adapter-node` and SSR. The
  Capacitor build keeps `@sveltejs/adapter-static`; the two artifacts remain
  separate.
- **Public identities:**
  - `currents.is` — website, same-origin browser API, OAuth client metadata,
    OAuth callback, and host-only web session cookie.
  - `api.currents.is` — stable AT Protocol appview service
    (`did:web:api.currents.is`) and public endpoint for native/extension clients.
  - `cdn.currents.is` — Bunny Pull Zone for immutable images and future
    frontend-generated OG images.
- **OAuth prompt:** the OAuth `client_id` is
  `https://currents.is/oauth-client-metadata.json`, with `client_name` set to
  `Currents` and `client_uri` set to `https://currents.is`. PDS authorization
  screens therefore identify `currents.is` (or `Currents` for a trusted client),
  not `api.currents.is`.
- **Cookies:** no parent-domain cookie. The web session cookie stays host-only
  on `currents.is`, `Secure`, `HttpOnly`, `Path=/`, and `SameSite=Lax`. This
  prevents it from being sent to Bunny at `cdn.currents.is` or to any other
  subdomain.
- **No SvelteKit BFF:** Caddy sends browser `/api`, `/xrpc`, and `/oauth`
  requests directly to appview. During SSR, SvelteKit forwards the incoming
  cookie to appview over the Docker network. SvelteKit does not own a second
  session or proxy browser API calls.
- **Image storage:** Currents continues to treat users' PDSes as the source of
  truth. Bunny is a pull cache, not permanent storage.
- **Mac mini:** it is not part of the final production topology. Keep its old
  database volume untouched for the initial rollback window, then retire it.

Approximate compute cost at the starting sizes is €42/month before tax,
Block Storage, Flexible IPs, Object Storage, and Bunny:

| Instance | Purpose | Approx. monthly |
|---|---|---:|
| BASIC2-A2C-8G | main stack | €25.18 |
| BASIC2-A2C-4G | CPU inference | €16.79 |

Prices are a provisioning-time check, not a promise; confirm them in the
Scaleway console before creating the instances.

## End state

```text
Cloudflare authoritative DNS
   production hosts: DNS-only direct A records
                         │
                         ▼
              Scaleway Flexible IPv4
                  public :80 / :443
                         │
                         ▼
Scaleway main instance   ┌─────────────┐
┌────────────────────────│    Caddy    │────────────────────────┐
│                        └──────┬──────┘                        │
│             page routes ──────┤────── /api, /xrpc, /oauth     │
│                    ▼          │                 ▼             │
│             SvelteKit SSR     │              appview ─────────┼──┐
│                    │          │                 │              │  │
│                    └──────────┴────────────▶ Postgres ◀── TAP  │  │
│                                              ▲                │  │
│                                         clustering ─────┐     │  │
└─────────────────────────────────────────────────────────┼─────┘  │
                                                          ▼        │
                                               Object Storage      │
                                               models + backups    │
                                                                   │
Scaleway Private Network                                          │
┌──────────────────────────────────────────────────────────────────┘
│   Scaleway inference instance
└──▶ FastAPI + SigLIP2 (CPU FP32, :8000 private only)

cdn.currents.is ──▶ DNS-only CNAME ──▶ Bunny ──▶ currents.is/img or /og

dev.* ──▶ Cloudflare Tunnel ──▶ development environments only
```

## 0. Required code and configuration work

These changes ship before either production cutover. Do not improvise them
during the database migration.

Implementation status (2026-08-15): the dual frontend build, public SSR loads
and metadata, appview identity split, `/api/*` aliases, host-only cookie
transition, service/CDN URL split, CPU dtype, two-phase Caddy configuration,
and both Scaleway Compose files are implemented and covered by local checks.
The CPU performance/RSS comparison and live Caddy/Bunny/DNS checks still
require the provisioned VMs and remain acceptance gates, not completed work.

### 0.1 Split the web and Capacitor builds

- Use `@sveltejs/adapter-node` for the normal web build.
- Keep `@sveltejs/adapter-static` with the existing SPA fallback when
  `CAPACITOR=1`.
- Remove the unconditional root `ssr = false`; disable SSR only for the
  Capacitor artifact.
- Add a frontend container to `docker-compose.scaleway.yml`, listening on
  port 3000 only inside the Compose network.
- Set adapter-node's trusted reverse-proxy origin configuration (`ORIGIN` or
  the forwarded host/protocol headers) explicitly.
- Move public profile, collection, and save data into universal/server-capable
  loads so their initial HTML can render dynamic `title`, `description`,
  `og:title`, `og:description`, and `og:image` values.
- Browser API URLs become relative. Native builds retain an absolute public
  API base URL.
- Server-side loads call `http://appview:8080` and forward the incoming Cookie
  header. Browser calls still go to relative same-origin paths through Caddy.

The first SSR release does not need to server-render viewer personalization.
Public page data and metadata are the SEO requirement; viewer-only state can
hydrate after mount until each store is made server-safe.

### 0.2 Split identities that `CLIENT_HOSTNAME` currently conflates

Replace the single hostname setting with distinct configuration:

```text
OAUTH_HOSTNAME=currents.is
SERVICE_HOSTNAME=api.currents.is
FRONTEND_URL=https://currents.is
CDN_URL=https://cdn.currents.is
```

The resulting values must be:

```text
OAuth client ID:  https://currents.is/oauth-client-metadata.json
OAuth callback:   https://currents.is/oauth/callback
OAuth client URI: https://currents.is
OAuth client name: Currents
Service DID:      did:web:api.currents.is
Service endpoint: https://api.currents.is
Image base URL:   https://cdn.currents.is
```

`GET https://currents.is/oauth-client-metadata.json` must answer `200` with
the metadata document directly; AT Protocol does not allow that fetch to be a
redirect. The JWKS endpoint and callback also live on `currents.is`.

Changing the OAuth client ID from `api.currents.is` to `currents.is` is a new
client registration from each PDS's perspective. Expect every user to approve
Currents again on their next login. It does not affect their PDS records or
indexed Currents data.

Keep the service DID independent. `/.well-known/did.json` on
`api.currents.is` continues to advertise `did:web:api.currents.is`; moving
OAuth must not change the service-auth audience.

### 0.3 Make browser API routes unambiguous

On `currents.is`, Caddy owns the split:

```text
/                                      -> frontend:3000
/api/*                                 -> appview:8080
/xrpc/*                                -> appview:8080
/oauth/*                               -> appview:8080
/oauth-client-metadata.json            -> appview:8080
/img/*                                 -> appview:8080
/.well-known/site.standard.publication -> frontend:3000
```

The existing top-level appview routes (`/save`, `/collection`, `/follow`,
`/favourite`, `/resave`) conflict with frontend routes such as
`/save/[saveId]` and `/collection/[uri]`. Give the first-party HTTP handlers
canonical `/api/*` aliases and update current clients to use them. Retain the
old routes on `api.currents.is` for released mobile and extension versions;
do not delete them as part of this migration.

On `api.currents.is`, Caddy sends normal API and XRPC traffic to appview.
Legacy `GET` or `POST /oauth/login` requests receive a method-preserving
redirect to the equivalent `https://currents.is/oauth/login` URL **before** an
OAuth flow begins. This lets already-released native builds start the new
root-domain OAuth flow while their authenticated API calls continue using
bearer tokens against `api.currents.is`.

### 0.4 Keep the session cookie host-only

Do not set `Domain=currents.is`. The appview response is served through the
`currents.is` Caddy route, so an omitted Domain attribute naturally creates
the correct host-only cookie.

Use:

```text
Path=/; Secure; HttpOnly; SameSite=Lax
```

The existing `SameSite=None` setting is unnecessary once browser requests are
same-origin. Capacitor uses bearer authentication after OAuth, so it does not
need a cross-site session cookie.

During the transition, logout must also visit an `api.currents.is`
compatibility endpoint that expires the old API-host cookie. A response from
`currents.is` cannot clear another host's host-only cookie.

### 0.5 Separate the appview service URL from the image CDN URL

`CDNBaseURL` currently also populates the appview DID document's
`serviceEndpoint`. Split those fields before enabling Bunny:

- appview DID `serviceEndpoint` -> `https://api.currents.is`
- XRPC image/avatar/preview URLs -> `https://cdn.currents.is/img/...`

Without this split, setting `CDN_URL` would incorrectly advertise Bunny as the
AT Protocol appview.

Create a Bunny Pull Zone with:

- custom hostname `cdn.currents.is`;
- origin `https://currents.is`;
- a DNS-only Cloudflare `CNAME` from `cdn` to the Bunny hostname;
- an explicit cache rule for `/img/*` because CID paths have no file extension;
- an explicit cache rule for `/og/*` when dynamic OG image generation ships;
- European Origin Shield;
- no cookie forwarding or cookie-based cache variation.

The appview already returns image responses as public, one-year, immutable
content. Bunny may evict and re-pull them; the source remains the user's PDS.

### 0.6 Make inference CPU-efficient and bounded

The current unconditional BF16 dtype is slow when a CPU does not accelerate
BF16. Make dtype device-dependent while preserving Apple Silicon support:

```python
DTYPE = torch.bfloat16 if DEVICE == "mps" else torch.float32
```

Start the 4 GB inference instance with:

```text
uvicorn workers:    1
IMAGE_MAX_BATCH:    4
IMAGE_QUEUE_SIZE:   8
device:             cpu
dtype:              float32
```

One worker matters: every Uvicorn worker would load another copy of SigLIP2.
Keep the Hugging Face cache and runtime models on persistent paths so container
rebuilds do not redownload the 1.4 GB checkpoint.

Before directing production appview traffic to it, test a representative
sample against the existing MPS server:

1. Compare CPU FP32 and MPS BF16 embeddings for the same images and texts.
   Self-pair cosine distance must be comfortably below the visual-identity
   dedup threshold (`0.02`); target `< 0.001`.
2. Verify representative search top results do not materially change.
3. Warm the model, then measure text p50/p95 and image throughput.
4. Run text queries while continuously embedding images. Text p95 should stay
   below one second and the image queue must drain after the burst.
5. Exercise the configured four-image batch plus a full decoded-image queue
   while watching RSS; the process should remain below 3 GB on the 4 GB
   instance.
6. Verify `/health` reports CPU, UMAP loaded, all deployed moderation heads,
   the junk head, and empty queues after the test.

If the latency or memory gate fails, resize inference to 4 vCPU / 8 GB. Do not
change embedding models or quantize the stored vector space merely to fit the
cheapest instance.

### 0.7 Expose Caddy directly and automate TLS

Use Caddy rather than nginx plus Certbot. It provides the required host/path
routing and automatic certificate issuance, renewal, and reload in one
self-hosted service.

- Give only Caddy host bindings for ports 80 and 443. Postgres, TAP, appview,
  SvelteKit, clustering, and inference stay on Docker or Private Network
  addresses.
- Persist Caddy's `/data` and `/config` directories so its ACME account and
  certificates survive container replacement.
- Keep port 80 open for ACME HTTP-01 and Caddy's HTTPS redirect. Caddy obtains
  certificates for `currents.is`, `api.currents.is`, and each public labeler
  hostname; Bunny manages the certificate for `cdn.currents.is`.
- Reject unknown `Host` values instead of routing them to appview or SvelteKit.
- Keep development tunnel hostnames separate. They may remain proxied tunnel
  records in Cloudflare, but no production hostname is configured on a tunnel.

Production records must use Cloudflare's **DNS only** mode. This exposes the
Scaleway IP by design and lets Caddy complete HTTP-01 without a Cloudflare API
token. At each first DNS cutover there may be a short certificate-issuance gap;
schedule it as part of the maintenance window and wait for Caddy to report a
valid certificate before declaring the service live.

## 1. Provision Scaleway resources

Create both instances in the same Paris region and attach them to one Private
Network.

### Main instance

- `BASIC2-A2C-8G` (or a dedicated-core equivalent if predictable DB latency
  later proves necessary), Ubuntu 24.04 LTS.
- One Flexible IPv4 kept independently of the VM so it can be reattached to a
  replacement instance in the same Availability Zone.
- 30 GB Block Storage volume for `/mnt/pgdata`.
- Security group: inbound TCP 80/443 from the Internet; inbound SSH only from
  trusted administration IPs. Apply the same policy in the host firewall.
- Only Caddy publishes Docker ports on the public interface.

### Inference instance

- `BASIC2-A2C-4G`, Ubuntu 24.04 LTS.
- At least 15–20 GB total disk for the OS, Docker layers, the Hugging Face
  checkpoint, UMAP, and ONNX heads.
- Security group: inbound SSH only. Do not publish port 8000 on the public
  interface. Bind it to the Private Network address or enforce the same rule in
  the host firewall.
- A small swap file is acceptable as an emergency OOM fuse, but active swapping
  is a failed capacity test, not normal operation.

### Shared services

- The existing Cloudflare DNS zone, with production records set to DNS-only
  and tunnel/proxied records reserved for development hostnames.
- `currents-db-backups` Object Storage bucket in `fr-par`, with a 30-day
  lifecycle rule.
- `currents-models` Object Storage bucket in `fr-par`, with versioning enabled.
  It carries the UMAP model and deployed classifier heads from clustering or
  deployment to inference.
- One API key scoped only to those buckets.
- One stable Private Network address or internal DNS name for inference.
- Bunny Pull Zone as specified in section 0.5. It can be enabled after the SSR
  cutover; it is not on the critical path for the database move.

## 2. Prepare and validate the inference instance

Install Docker, clone the repository to `/opt/currents`, and deploy the
single-service inference Compose file created in section 0.6.

Use separate persistent directories, for example:

```text
/opt/currents-inference/huggingface  # checkpoint cache
/opt/currents-inference/models       # UMAP and ONNX heads
```

Set at least:

```text
MODELS_DIR=/models
HF_HOME=/huggingface
IMAGE_MAX_BATCH=4
IMAGE_QUEUE_SIZE=8
```

Seed the runtime models from Object Storage, start the service, run the full
acceptance test in section 0.6, and record the actual warm latency, throughput,
and peak RSS in this document before production cutover.

Add a daily model sync on the inference host. The sync must download a complete
object before atomically replacing the active local file, then call
`POST http://127.0.0.1:8000/reload-umap`. A partially downloaded UMAP file must
never replace the live one.

Keep the mac mini inference server available as rollback until the Scaleway CPU
server has passed both the synthetic test and a production soak.

## 3. Prepare the main instance

Install Docker and attach the Block Storage volume. Resolve the device with
`lsblk` before formatting it; never copy a device name from this document
blindly.

```bash
# Example only after verifying the exact empty Block Storage device:
mkfs.ext4 /dev/<verified-block-device>
mkdir -p /mnt/pgdata
echo '/dev/<verified-block-device> /mnt/pgdata ext4 defaults,nofail 0 2' >> /etc/fstab
mount -a

git clone https://github.com/<you>/currents.git /opt/currents
```

For the first cutover, keep the existing public behavior:

```text
OAUTH_HOSTNAME=api.currents.is
SERVICE_HOSTNAME=api.currents.is
FRONTEND_URL=https://currents.is
CDN_URL=https://api.currents.is
INFERENCE_URL=http://<inference-private-address>:8000
MODELS_S3_BUCKET=currents-models
```

Also copy the existing secrets (`SESSION_SECRET`, client signing key,
`TAP_ADMIN_PASSWORD`, `POLAR_*`, labeler configuration) unchanged. A changed
session secret would invalidate every Currents session independently of the
planned OAuth client-ID change.

`DB_PASSWORD` is required on the Scaleway compose. Postgres publishes no port;
administrative access goes through SSH and `docker compose exec`.

Validate the Caddyfile and test every upstream over the internal Compose
network. Do not add the production host blocks early: enable them immediately
before their DNS-only records move to the Flexible IPv4 so Caddy can complete
ACME validation. Do not cut `currents.is` away from Netlify yet.

The Compose default mounts `Caddyfile.scaleway.phase-a`, which exposes only
`api.currents.is` and does not redirect OAuth. Keep `CADDYFILE` unset during
Cutover A.

## 4. Cutover A: database, TAP, appview, and inference

One dump/restore carries appview tables and TAP's tables (`repos`,
`repo_records`, `firehose_cursors`, outbox). Because the firehose cursor moves
with the database, TAP resumes from that point and replays the short gap from
the relay.

```bash
# On the mac mini: stop writers, keep db up.
docker compose -f docker-compose.mac-mini.yml stop appview tap clustering
docker exec currents-db-1 pg_dump -U appview -d appview -Fc > currents.dump

# Ship over SSH during the migration window.
scp currents.dump root@<main-vm-address>:/opt/currents/

# On the main VM: start a fresh DB and restore.
cd /opt/currents
docker compose -f docker-compose.scaleway.yml up -d db
docker compose -f docker-compose.scaleway.yml exec -T db \
  pg_restore -U appview -d appview --no-owner < currents.dump

# Seed/publish the current UMAP model before starting appview and clustering.
scp <mini>:<currents-path>/models/umap_model.joblib /opt/currents/models/
aws --endpoint-url https://s3.fr-par.scw.cloud \
  s3 cp /opt/currents/models/umap_model.joblib s3://currents-models/

docker compose -f docker-compose.scaleway.yml up -d --build \
  appview tap clustering caddy
```

Sanity-check before changing public DNS:

```bash
docker compose -f docker-compose.scaleway.yml exec db \
  psql -U appview -d appview -c "SELECT count(*) FROM save;" \
                             -c "SELECT count(*) FROM visual_identity;" \
                             -c "SELECT * FROM firehose_cursors;"
```

Enable and reload the Caddy host blocks for `api.currents.is` and each public
labeler hostname, then change their Cloudflare records to direct, DNS-only `A`
records for the Flexible IPv4. Watch the Caddy logs until certificates are
issued, verify HTTPS reaches the new instance, then remove those production
hostnames from the old tunnel. Development tunnel hostnames remain untouched.
There is no tunnel service on the production Scaleway VMs.

Verify:

- existing Netlify web login and authenticated requests still work;
- appview reaches inference over the Private Network;
- text search latency is acceptable while saving images;
- a new save produces a visual identity, palette, moderation scores, and junk
  score;
- TAP logs the save and its cursor advances between two checks;
- Polar webhook and supporter status still work;
- `did:web:api.currents.is` and service-auth XRPC still work;
- `dig` returns the Flexible IPv4 and production responses do not contain a
  Cloudflare proxy header such as `cf-ray`;
- admin and labeler endpoints still work.

Soak this state before the SSR/OAuth cutover. Run the repair pass for any
`visual_identity_id IS NULL` rows created during migration.

### Cutover A rollback

Stop the VM appview/TAP/clustering, restore the latest VM dump into the mac
mini database if the VM accepted writes, restart the mac stack, and restore the
previous API/labeler tunnel records and routes. Keep the old mac `pgdata`
volume untouched for at least two weeks.

## 5. Cutover B: SvelteKit SSR and root-domain OAuth

Only begin after Cutover A has soaked successfully.

Before DNS changes:

1. Deploy the frontend Node container on the main VM.
2. Validate the Caddy host/path routing from section 0.3, but do not enable its
   production `currents.is` host block yet.
3. Prepare the DNS-only root `A` record change without applying it.
4. Test the frontend container and appview routes over the internal Compose
   network.
5. Verify `api.currents.is/oauth/login` has the legacy redirect ready, but do
   not enable it while the OAuth client still points at `api.currents.is`.

At cutover:

1. Change appview to `OAUTH_HOSTNAME=currents.is` and restart it.
2. Set `CADDYFILE=./Caddyfile.scaleway` and recreate Caddy. This enables both
   the legacy `api.currents.is/oauth/login` redirect and the `currents.is` host.
3. Replace the Netlify root record with
   a DNS-only `A` record for the Scaleway Flexible IPv4.
4. Wait for Caddy to issue the certificate, then verify the root OAuth metadata
   returns `200` directly and contains the exact
   root-domain client ID/callback values.
5. Log in again and confirm the PDS prompt identifies `currents.is` or
   `Currents`.
6. Confirm the new cookie has no Domain attribute and is not sent on requests
   to `api.currents.is` or `cdn.currents.is`.

A brief login interruption is preferable to running both OAuth client
identities concurrently. Existing users will need to approve the new client.

After SSR and OAuth are stable:

1. Create/test the Bunny Pull Zone against the `/img` origin using its default
   `b-cdn.net` hostname.
2. Attach `cdn.currents.is`, enable TLS, and verify cache MISS then HIT behavior.
3. Change `CDN_URL=https://cdn.currents.is` and restart appview.
4. Verify the appview DID still advertises `https://api.currents.is`, not the
   CDN hostname.
5. Add `/og/*` to Bunny only when the frontend OG image endpoint ships.

### Cutover B verification

- `view-source:` for a public profile, collection, and save contains meaningful
  HTML plus route-specific title, description, canonical URL, and OG fields.
- Sharing a save/collection resolves an absolute, publicly fetchable OG image.
- Logged-in browser reads and mutations use relative same-origin routes.
- SSR can authenticate an incoming root-domain cookie against internal appview.
- Logout clears both the new root cookie and any stale API-host cookie.
- Capacitor login through the legacy API URL redirects to root OAuth and returns
  to the app with a bearer token.
- Existing native API calls and the browser extension still work against
  `api.currents.is`.
- `https://api.currents.is/.well-known/did.json` has ID
  `did:web:api.currents.is` and endpoint `https://api.currents.is`.
- `https://currents.is/oauth-client-metadata.json` has client name `Currents`,
  root client URI/callback, and a working root JWKS URI.
- `cdn.currents.is/img/...` returns the same bytes/content type as origin,
  includes cache headers, and never varies on Cookie.

### Cutover B rollback

Restore the previous Netlify DNS record for `currents.is`, revert
`OAUTH_HOSTNAME` to `api.currents.is`, disable the legacy OAuth redirect, and
temporarily restore `CDN_URL=https://api.currents.is`. The root-domain OAuth
grants may remain at PDSes but become unused; no Currents data rollback is
necessary.

## 6. Retire the mac mini production stack

After both cutovers and the inference soak:

```bash
docker compose -f docker-compose.mac-mini.yml down
```

Keep its old database volume untouched for the agreed rollback window. After
that window, verify a fresh Object Storage restore before reclaiming the old
volume. Removing it is a separate destructive operation and is not part of
this migration procedure.

The inference instance now receives model updates from Object Storage. The mac
mini no longer runs a sync cron or any production service.

## 7. Backups

Three layers, cheapest first:

1. **Nightly `pg_dump` to Object Storage.** On the main VM:

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

   Configure the bucket-scoped key and run at `10 4 * * *`. The bucket's
   30-day lifecycle rule handles retention.

2. **Weekly Block Storage snapshot** of `/mnt/pgdata`.

3. **Network-derived recovery.** Saves, collections, follows, and profiles can
   rebuild from PDSes through TAP; embeddings, palettes, and classifier scores
   can be backfilled. The irreplaceable small tables (moderation/review state,
   `seen_feature`, `moderation_pref`, `starred_collection`, `color_trial`) make
   the nightly database dump mandatory despite most rows being derived.

Test a complete restore before deleting the mac rollback volume and at regular
intervals afterward. A backup that has never been restored is unverified.

## 8. Add CI/CD after the migration is stable

Do this only after both cutovers have soaked, the production backup has passed
a restore test, and the mac mini rollback window has ended. The migration's
first production deployments remain manual so CI/CD is not another cutover
variable.

### Continuous integration

Add GitHub Actions on pull requests and `main`:

- appview: `go test ./...`, both plain and against a `pgvector/pgvector`
  service using `TEST_DATABASE_URL`;
- frontend: `npm run check`, `npm run lint`, `npm run test:unit`, and successful
  builds of both the SSR web artifact and the Capacitor artifact;
- inference: `python -m unittest -v` without downloading or loading SigLIP2;
- clustering: its unittest suite in the clustering Docker image.

Keep Playwright out of the required workflow initially. Add it after the SSR
mocked flows are stable and its browser/runtime cost is justified.

### Image publishing

After CI passes on `main`, build the appview, frontend, inference, and
clustering images once and push them to a private Scaleway Container Registry
namespace in `fr-par`.

- Tag every image with the full Git commit SHA. Never deploy `latest`.
- Give CI a registry-push credential scoped to this purpose. Give the two VMs
  a separate pull credential; do not copy the CI credential to production.
- Keep model files out of the inference image. They continue to arrive through
  the persistent Hugging Face cache and `currents-models` bucket.
- Pin third-party production images such as Postgres, TAP, and Caddy to explicit
  versions or digests; do not rebuild them in Currents CI.
- Change the production Compose files from local `build:` entries to image
  references parameterized by a single `RELEASE_SHA`.

### Controlled production delivery

Use a protected GitHub `production` environment and require approval before
the deploy job receives its secrets. The first version deploys over SSH; it
does not need Kubernetes, a self-hosted runner, or an agent on either VM.

- Create a non-root deployment user whose SSH key is forced to run one
  root-owned deploy script. Do not give it an interactive shell or general
  access to the Docker socket.
- Keep application secrets exclusively in root-readable environment files on
  the VMs. CI receives only the registry credential, deployment SSH key, host
  fingerprints, and host addresses.
- Serialize releases with a host-side lock. Reject a deploy if another release
  or backup is active.
- Deploy inference first when it changed, wait for its private `/health`, then
  deploy the main VM. Pull the exact SHA and recreate only changed application
  services; never restart Postgres, TAP, or Caddy merely because a release ran.
- Before a release containing migrations, take a fresh database dump. Schema
  changes must remain compatible with the previous appview image for one
  rollback. Never run a down migration automatically.
- Verify inference health, appview health/XRPC, one SSR page, and the public
  OAuth metadata after deployment. Record the deployed SHA and timestamp.
- On failed health checks, restore the previous image SHA and recreate the
  affected application services. If a forward database migration makes that
  unsafe, stop automatic rollback and require a manual recovery decision.

Start as continuous **delivery**: merges publish immutable images, while the
production rollout requires approval. Automatic deployment on every `main`
push can be reconsidered only after the approved workflow has proved reliable.

## Ops notes

- **Main instance resize:** stop, change type, and start. The DB lives on
  detachable Block Storage. Raise `shared_buffers` to roughly 25% of RAM after
  a RAM increase.
- **Inference resize:** resize independently when its measured latency, queue,
  or RSS crosses the acceptance gate. No database move is involved.
- **Inference unavailable:** novel saves remain indexed with
  `visual_identity_id IS NULL`; run the repair pass after recovery. This is why
  the bounded 4 GB configuration returns 503 instead of risking a host OOM.
- **TAP desynchronization:** mark the affected repo `desynchronized` with zeroed
  retry fields; the resyncer re-fetches and diff-emits it.
- **Bad UMAP retrain:** restore the previous version in `currents-models`, sync
  it to the inference instance, call `/reload-umap`, and rerun re-projection
  after a good model is published.
- **Bunny unavailable:** temporarily set `CDN_URL=https://api.currents.is`;
  appview remains a functional image origin.
- **Single failure domain:** the main VM still takes the website, API, TAP, and
  DB offline together. At the current scale this is an accepted cost tradeoff,
  mitigated by Block Storage, snapshots, Object Storage dumps, and a tested
  restore. `SCALING.md` defines when to resize or split it.
