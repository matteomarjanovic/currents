# Scaleway CI/CD

The frontend remains on Netlify. Netlify deploys it from `main`; the workflows
here validate the web and Capacitor artifacts but publish and deploy only
appview, inference, and clustering.

## Release flow

1. `.github/workflows/ci.yml` runs on pull requests and `main`.
2. A green `main` run starts `publish-images.yml`, which builds ARM64 images
   tagged with the full commit SHA in the private `fr-par` registry.
3. `deploy-production.yml` is started manually with that SHA. The protected
   `production` environment withholds its SSH secrets until approval.
4. Inference is deployed first when selected. The main host then recreates
   only appview and/or clustering; DB, TAP, Caddy, and the Netlify frontend are
   never recreated by the release.

The host scripts serialize releases, pull exact tags, retain the prior image,
and roll back a service that fails health checks. Every appview deployment
takes a fresh Object Storage database dump first. A dirty migration stops the
script without attempting an unsafe automatic rollback; down migrations are
never run.

## One-time registry setup

Create a private Container Registry namespace in `fr-par` and two separate IAM
API keys:

- a CI key that can push images;
- a production key that can only pull images.

Set these repository Actions values:

| Kind | Name | Value |
|---|---|---|
| Variable | `SCW_REGISTRY` | `rg.fr-par.scw.cloud/<private-namespace>` |
| Secret | `SCW_REGISTRY_SECRET_KEY` | secret part of the CI push key |
| Secret | `CAPAWESOME_TOKEN` | existing Capawesome token, so internal CI can build the native artifact |

The publish workflow is deliberately skipped while `SCW_REGISTRY` is absent.
It uses `nologin` as the registry username, as documented by Scaleway.

On each VM, log root's Docker client into the registry with the separate pull
key, then create a root-only registry file:

```sh
printf '%s\n' '<pull-secret-key>' | docker login rg.fr-par.scw.cloud -u nologin --password-stdin
install -m 600 -o root -g root /dev/null /opt/currents/.env.registry
# Add exactly: SCW_REGISTRY=rg.fr-par.scw.cloud/<private-namespace>
```

Do not put either registry key in the repository or the normal application env
files.

## One-time restricted SSH setup

Generate one dedicated Ed25519 deployment key. Put its private half in the
`production` environment secret `PRODUCTION_DEPLOY_SSH_KEY`; it must not be
used for administration. Install the public half for a locked
`currents-deploy` user on both VMs.

On the main VM, install the scripts from the checked-out repository:

```sh
install -o root -g root -m 755 deploy/scaleway/deploy-main-release.sh /usr/local/sbin/currents-deploy-main
install -o root -g root -m 755 deploy/scaleway/forced-main-command.sh /usr/local/sbin/currents-deploy-main-ssh
```

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

Create an environment named `production`, add a required reviewer, prevent
self-review when another reviewer is available, and restrict it to `main`.
Add:

| Kind | Name | Value |
|---|---|---|
| Variable | `MAIN_DEPLOY_HOST` | `51.159.84.247` |
| Variable | `INFERENCE_DEPLOY_HOST` | `51.159.87.81` |
| Secret | `PRODUCTION_DEPLOY_SSH_KEY` | dedicated private key |
| Secret | `PRODUCTION_KNOWN_HOSTS` | verified `known_hosts` lines for both IPs |

Verify both host-key fingerprints against the Scaleway console before storing
the `known_hosts` lines. Do not accept a new key merely because `ssh-keyscan`
returned it. This is especially important after an instance replacement.

Protect `main` with the five CI job names: `Appview`, `Appview DB`, `Frontend`,
`Inference`, and `Clustering`. Keep Playwright optional for now.

## First release

After the settings and both hosts are ready:

1. Run **Publish images** manually for the current full `main` SHA.
2. Confirm all three exact-SHA tags exist in the registry.
3. Run **Deploy production** with that SHA and approve the `production` gate.
4. Verify `/var/lib/currents-releases/*.current` and the public checks.

The production Compose files keep their local `build:` definitions as a manual
recovery fallback. Automated deploys always set the exact registry image and
use `--no-build`.
