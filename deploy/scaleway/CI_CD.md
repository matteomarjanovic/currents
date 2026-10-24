# Scaleway CI/CD

After Cutover B, the frontend runs on the Scaleway main VM. The workflows then
publish and deploy it alongside appview, inference, and clustering. Capacitor
releases continue to be built locally.

## eMudhra CA compromise (October 2026)

Treat this as an urgent outbound HTTPS client update. See the
[incident and mitigation tracker](https://emudhra.we-have-a-problem.com/) and
[Go security release](https://groups.google.com/g/golang-announce/c/66pojueaQek).
The appview now requires Go 1.26.10, its Debian image checks for the patched
CA package, and both Python services require certifi 2026.10.19 or newer.
Publish and deploy all three changed services; source changes do not patch
running containers.

The ordinary release does **not** update the complete stack:

- Update the system CA packages on both VMs and restart clients using the old
  trust store. Host updates do not update container trust stores.
- TAP and Caddy mount the host's `/etc/ssl/certs` read-only, replacing their
  stale image trust stores. Apply the updated compose file on the main VM and
  recreate these two services separately; the release workflow excludes them.
  Mount the whole directory so Go cannot re-add old roots from certificate
  files alongside the bundle. Restart both after future host CA updates.
- The AWS CLI wrappers for backup, backup verification, and model sync also
  mount that directory and set `AWS_CA_BUNDLE`. Install the updated scripts on
  their hosts; the release workflow does not install host scripts.
- The frontend image requires Alpine's patched CA package and starts Node with
  `--use-openssl-ca` and an explicit `SSL_CERT_FILE`, replacing its bundled
  CA snapshot for both builds and SSR. Deploy the changed frontend image.

Verify each relevant HTTPS client rejects the compromised chains using the
tracker's live endpoints or offline chains, while ordinary requests still
succeed. A DNS, timeout, or connection error is not a successful distrust test.

## Release flow

1. `.github/workflows/ci.yml` runs on pull requests and `main`.
2. A green `main` run starts `publish-images.yml`, which builds ARM64 images
   only for changed services, tagged with the full commit SHA in the private
   `fr-par` registry. Publishing runs on native `ubuntu-24.04-arm` runners;
   production ARM64 builds do not use QEMU emulation.
3. A successful image publish starts `deploy-production.yml`. It deploys those
   same changed services automatically.
4. Inference is deployed first when it changed. The main host then recreates
   only the changed appview, clustering, and/or frontend service; DB, TAP, and
   Caddy are never recreated by the release.

The host scripts serialize releases, pull exact tags, retain the prior image,
and roll back a service that fails health checks. Every appview deployment
takes a fresh Object Storage database dump first. A dirty migration stops the
script without attempting an unsafe automatic rollback; down migrations are
never run. A health failure also refuses rollback if the schema version changed
or its state cannot be confirmed. The new image is retained for operator
recovery: an older binary may not even start once a new migration is applied.

Internal HTTP probes use a five-second timeout and one attempt. Public metadata
checks allow five seconds; the feed query allows thirty seconds because a cold
production query can exceed five seconds. Public probe failures print the URL
and timeout so the failing check is visible in Actions logs. Readiness retries
have a two-minute elapsed-time budget; an in-flight bounded probe may finish
after that deadline. The timeout is not multiplied by sixty retry attempts.

The workflow sends SSH keepalives while the backup runs, so a quiet but healthy
backup cannot be mistaken for a dropped deployment connection.

The main-host script must use `/opt/currents/.env.production`, with
`OAUTH_HOSTNAME=currents.is`. `.env.production.phase-a` is historical rollback
configuration: using it during a normal release restores the API-host OAuth
callback while Caddy still starts login on the root host, losing the host-only
return cookie. The appview health check also verifies the root OAuth hostname.
Changes to the host scripts must be installed on the VM using the commands
below; pushing a commit deploys service images, not these installed scripts.

## One-time registry setup

Create a private Container Registry namespace in `fr-par` and two separate IAM
API keys:

- a CI key that can push images;
- a production key that can only pull images.

Set these Actions values:

| Kind | Name | Value |
|---|---|---|
| Variable | `SCW_REGISTRY` | `rg.fr-par.scw.cloud/<private-namespace>` |
| Secret | `SCW_REGISTRY_SECRET_KEY` | secret part of the CI push key |
| Production-environment secret | `SCW_REGISTRY_PULL_SECRET_KEY` | secret part of the VM pull key |

The publish workflow is deliberately skipped while `SCW_REGISTRY` is absent.
It uses `nologin` as the registry username, as documented by Scaleway.

On each VM, create a root-only registry file:

```sh
install -m 600 -o root -g root /dev/null /opt/currents/.env.registry
# Add exactly: SCW_REGISTRY=rg.fr-par.scw.cloud/<private-namespace>
```

Each release passes the production pull key to its restricted deploy command on
standard input. The host uses a temporary Docker config only for exact-tag
pulls, then erases it; no registry key is persisted on either VM. Until the
separate environment secret is added, deployments fall back to the CI key, so
the first deployment can proceed but this fallback should be short-lived.

## One-time restricted SSH setup

Generate one dedicated Ed25519 deployment key. Put its private half in the
`production` environment secret `PRODUCTION_DEPLOY_SSH_KEY`; it must not be
used for administration. Install the public half for a locked
`currents-deploy` user on both VMs.

Keep root administration public keys in Scaleway instance tags or
`/root/.ssh/instance_keys` (root-owned, mode `600`), and verify access before
host maintenance. Scaleway regenerates `/root/.ssh/authorized_keys` at boot
and when its ecosystem package restarts the key-fetch service; keys added
only to that generated file can disappear during an ordinary package update.
Keep deployment keys under the restricted `currents-deploy` account below.

On the main VM, install the scripts from the checked-out repository:

```sh
install -o root -g root -m 755 deploy/scaleway/deploy-main-release.sh /usr/local/sbin/currents-deploy-main
install -o root -g root -m 755 deploy/scaleway/forced-main-command.sh /usr/local/sbin/currents-deploy-main-ssh
install -d -m 755 /etc/systemd/system/docker.service.d
install -o root -g root -m 644 deploy/scaleway/docker-pgdata.conf /etc/systemd/system/docker.service.d/currents-pgdata.conf
systemctl daemon-reload
```

The Docker drop-in requires `/mnt/pgdata` to be mounted before Docker starts.
This prevents PostgreSQL from starting against an empty directory on the root
disk if the database volume is late or unavailable at boot. Verify the volume
with `mountpoint /mnt/pgdata` before starting Docker; reloading systemd does
not restart running containers.

On the inference VM:

```sh
install -o root -g root -m 755 deploy/scaleway/deploy-inference-release.sh /usr/local/sbin/currents-deploy-inference
install -o root -g root -m 755 deploy/scaleway/forced-inference-command.sh /usr/local/sbin/currents-deploy-inference-ssh
```

On each host, create `currents-deploy` with a locked password and no Docker
group membership. Its key line must force the matching wrapper and disable all
other SSH facilities:

```text
restrict,command="/usr/local/sbin/currents-deploy-<main|inference>-ssh" ssh-ed25519 <public-key>
```

Allow only the matching validated root script through `/etc/sudoers.d/`:

```text
# main VM
currents-deploy ALL=(root) NOPASSWD: /usr/local/sbin/currents-deploy-main *

# inference VM
currents-deploy ALL=(root) NOPASSWD: /usr/local/sbin/currents-deploy-inference *
```

Validate each file with `visudo -cf <file>`. The user needs `/bin/sh` so sshd
can execute the forced command, but the key cannot open an interactive shell.

## GitHub production environment

Create an environment named `production`, leave its protection rules empty so
deployments remain automatic, and restrict it to `main`. Add:

| Kind | Name | Value |
|---|---|---|
| Variable | `MAIN_DEPLOY_HOST` | `51.159.84.247` |
| Variable | `INFERENCE_DEPLOY_HOST` | `51.159.87.81` |
| Secret | `PRODUCTION_DEPLOY_SSH_KEY` | dedicated private key |
| Secret | `PRODUCTION_KNOWN_HOSTS` | verified `known_hosts` lines for both IPs |

Verify both host-key fingerprints against the Scaleway console before storing
the `known_hosts` lines. Do not accept a new key merely because `ssh-keyscan`
returned it. This is especially important after an instance replacement.

Protect `main` with the six CI job names: `Appview`, `Appview DB`, `Frontend`,
`Inference`, `Clustering`, and `Deployment scripts`. Keep Playwright optional for now.

## First release

The initial baseline release completed on 2026-08-29 at
`46a1151eab1eacdfd5c65a9e07d0e527423c373d`.

After the settings and both hosts are ready, establish the first image set:

1. Run **Publish images** manually for the current full `main` SHA with
   **Publish every service** selected.
2. Confirm all three exact-SHA tags exist in the registry.
3. Run **Deploy production** with that SHA once.
4. Verify `/var/lib/currents-releases/*.current` and the public checks.

After that baseline release, every green `main` change under `appview/`,
`inference/`, `clustering/`, or `frontend/` deploys the matching service
automatically. An appview migration is therefore released automatically after
its staging validation and CI pass.

The production Compose files keep their local `build:` definitions as a manual
recovery fallback. Automated deploys always set the exact registry image and
use `--no-build`.

## Admin capacity reporting

The private `/admin` dashboard receives a signed snapshot from each production
VM once per minute. It is deliberately a one-way report: the dashboard never
gets SSH or Docker credentials.

Generate one shared value with `openssl rand -hex 32`. Add it as
`OPS_REPORTING_SECRET` to the production source used by
`prepare-production-env.sh`, regenerate the main VM compose environment, and
put the same value in `/etc/currents/ops.env` on both hosts. The file is
root-only (`install -d -m 700 /etc/currents`; `install -m 600 /dev/null
/etc/currents/ops.env`) and has this shape:

```sh
OPS_REPORT_URL=https://api.currents.is/internal/ops/snapshot
OPS_REPORTING_SECRET=<the shared random value>
OPS_HOST=main                         # inference on the inference VM
OPS_DISK_PATH=/mnt/pgdata             # /opt/currents-inference on inference
# Inference VM only:
OPS_MODEL_VERSION_FILE=/opt/currents-inference/models/.umap_model.joblib.version
```

Copy the four host-metrics files from `deploy/scaleway/` onto each VM and, as
root, run:

```sh
./install-host-metrics.sh /path/to/deploy/scaleway
systemctl start currents-host-metrics.service
systemctl status currents-host-metrics.timer
```

The appview ingestion route stays disabled until its own
`OPS_REPORTING_SECRET` is present. The backup, UMAP-training, and daily
clustering jobs record their last terminal run in PostgreSQL; the first
pre-migration backup may have no dashboard row, which is expected.
