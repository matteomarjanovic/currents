# Scaleway Cutover B handoff

Point-in-time handoff written on 2026-08-17. Revalidate every live fact before
changing production. `SCALEWAY_MIGRATION.md` remains the architectural and
rollback authority; this file records the exact checkpoint and the shortest
safe path to resume.

## Copy-paste prompt for a new conversation

```text
Continue the Currents Scaleway migration from Cutover B. The repository is
/Users/matteo/Projects/curr and the production main VM is
root@51.159.84.247. The inference VM is root@51.159.87.81, but Cutover B
should not require changing it.

Before acting, read AGENTS.md completely, then read
SCALEWAY_CUTOVER_B_HANDOFF.md and SCALEWAY_MIGRATION.md (especially sections
0, 4, and 5, including verification and rollback). Check BACKLOG.md too. Treat
those documents as the authority and do not redesign the settled topology.

Goal: finish Cutover B so the SvelteKit adapter-node SSR frontend, root-domain
OAuth client, and same-origin web cookie run on the Scaleway main VM behind
Caddy. Keep did:web:api.currents.is and native/extension API traffic on
api.currents.is. Keep images on cdn.currents.is through Bunny. Do not use a
Cloudflare Tunnel or Cloudflare proxy for production; Cloudflare remains DNS
only for production records.

Current checkpoint (revalidate it): Cutover A is complete and has been live on
Scaleway since 2026-08-17. api.currents.is points directly to 51.159.84.247.
currents.is still serves the working Netlify SSR deployment. Bunny is live at
cdn.currents.is, appview already emits CDN URLs, and optimized responsive
images plus dynamic OG metadata are live on Netlify. The Bunny implementation
landed on main in c7461cf. The main VM is currently running the Phase A Caddy
and appview identity from /opt/currents/.env.production.phase-a. The prepared
/opt/currents/.env.production switches only OAuth/Caddy to the final
root-domain configuration. Both env files are root-owned mode 0600. The VM's
/opt/currents is a deployed source snapshot without .git, so do not run git
pull there and never overwrite or expose .env*, .env.storage, models, database
dumps, or Caddy data.

Work autonomously through all safe preparation and verification. Start with a
read-only audit of local git state, live DNS/HTTP, VM services/logs/resources,
TAP cursor movement, the latest backup, and pending visual identities. Compare
the actual VM snapshot with current main before syncing tracked source. Use a
staged/inspectable transfer that preserves all server-only secrets and data.
Before rebuilding, tag or record the current appview/frontend image IDs for
rollback. Build and test the current adapter-node frontend on the VM while
Phase A remains public. Validate the final Caddyfile and its route split
without enabling it early.

Do not restart or recreate Postgres, TAP, clustering, or inference for this
cutover. Do not restore a database or run down migrations. Do not change the
service DID, SERVICE_HOSTNAME, CDN_URL, SESSION_SECRET, database credentials,
or signing keys. Do not expose secrets in command output. Preserve the
working Netlify site as the DNS rollback target.

Before the irreversible traffic switch, record the exact existing Cloudflare
root DNS record/Netlify target and confirm a current verified database backup.
Then follow the documented short cutover order: recreate appview using
.env.production, recreate Caddy using .env.production, change the apex/root
record to a DNS-only A record for 51.159.84.247 with no stale CNAME or AAAA,
wait for Caddy TLS, and run every Cutover B verification. Keep the interruption
between the OAuth identity change and DNS switch as short as possible.

I can perform Cloudflare dashboard changes or an interactive browser login if
you cannot. Do all other preparation first, then ask me only at the precise
point where that external action is required and give me exact instructions.
After the switch, have me test login and the PDS consent prompt while you
continue technical checks. Confirm the prompt identifies Currents/currents.is,
the cookie is host-only on currents.is, stale api.currents.is logout cleanup
works, Capacitor/legacy OAuth redirects correctly, extension/native API calls
still work, SSR/OG metadata remains correct, Bunny still returns optimized
MISS/HIT variants, and appview/TAP/inference remain healthy.

If a gate fails, use the documented Cutover B rollback immediately; do not
improvise around OAuth, DNS, or data. Bunny is independent from the root
cutover and should remain enabled unless Bunny itself is the failing component.
Once stable, update SCALEWAY_MIGRATION.md and BACKLOG.md with evidence, but do
not retire the mac mini until the agreed rollback window has elapsed. CI/CD is
a later backlog item, not part of this cutover.
```

## Exact checkpoint

### Public traffic

- `currents.is`: Netlify SvelteKit SSR, currently the production web frontend.
- `api.currents.is`: DNS-only direct traffic to the Scaleway Flexible IPv4
  `51.159.84.247`; Caddy forwards it to appview.
- `cdn.currents.is`: Bunny Pull Zone whose origin is
  `https://api.currents.is`; raw CID images and Dynamic Image variants are
  working.
- Service identity remains `did:web:api.currents.is` with endpoint
  `https://api.currents.is`.
- The inference service is private at `http://172.16.8.2:8000` from the main
  VM and is not part of the Cutover B traffic switch.

### Code and frontend

- `main` contains the working Netlify SSR adapter fix and Bunny image work.
- Bunny implementation commit: `c7461cf` (`Use Bunny CDN image optimization`).
- At creation time this handoff and its small migration-plan corrections are
  local documentation changes. Preserve them if `git status` still shows them;
  do not treat them as unrelated dirty-worktree changes.
- The live Netlify SSR source was verified to contain a resized OG URL and
  responsive `srcset`. A production 1200x1600 variant returned WebP with a
  Bunny MISS followed by HIT.
- The same source still builds the Capacitor static artifact separately.
- Before Cutover B, rerun the frontend unit tests, `npm run check`, the
  Capacitor build, and the adapter-node production build from current `main`.

### Main VM

At handoff time all six Compose services were running: `db`, `tap`, `appview`,
`frontend`, `clustering`, and `caddy`. PostgreSQL was healthy and the nightly
backup timer was active.

Active `/opt/currents/.env.production.phase-a` routing values:

```text
OAUTH_HOSTNAME=api.currents.is
SERVICE_HOSTNAME=api.currents.is
FRONTEND_URL=https://currents.is
CDN_URL=https://cdn.currents.is
ORIGIN=https://currents.is
CADDYFILE=./Caddyfile.scaleway.phase-a
```

Prepared `/opt/currents/.env.production` routing values:

```text
OAUTH_HOSTNAME=currents.is
SERVICE_HOSTNAME=api.currents.is
FRONTEND_URL=https://currents.is
CDN_URL=https://cdn.currents.is
ORIGIN=https://currents.is
CADDYFILE=./Caddyfile.scaleway
```

The deployment directory is **not a Git checkout**. It contains production-only
files including sealed env files, Object Storage credentials, model artifacts,
and large database dumps. A future deploy must transfer only reviewed tracked
source. Do not use a broad `rsync --delete` or replace `/opt/currents`.

### Existing rollback material

- Netlify remains live until the root DNS switch and is the frontend rollback.
- `.env.production.phase-a` restores the old OAuth identity and Phase A Caddy.
- Pre-Bunny sealed env backups exist on the VM, but Bunny is now proven and is
  not normally part of a Cutover B rollback.
- Caddy certificate/account state persists in Docker volumes.
- The mac mini production database volume is retained only for the wider
  Cutover A rollback window; Cutover B itself requires no data migration.

## Resume checklist

Use this list as a gate, not as a substitute for the detailed migration plan.

1. Inspect local `main`, working-tree ownership, remote head, and all relevant
   tests/builds. Preserve the intentional handoff documentation changes above.
2. Audit VM health, recent errors, TAP cursor movement, disk/memory, backup
   timer, latest backup verification, and enrichment backlog.
3. Record current Docker image IDs and the exact Cloudflare root DNS/Netlify
   rollback value.
4. Transfer reviewed tracked source to `/opt/currents` without touching any
   server-only or secret files.
5. Build/recreate only `frontend` while Phase A stays public; test SSR and
   internal appview access over the Compose network.
6. Validate the final Compose expansion and `Caddyfile.scaleway` without
   activating root-domain routing.
7. Agree on a short user-attended window for the DNS change and login test.
8. Recreate only `appview` and `caddy` with `.env.production`, then immediately
   switch the root record to the DNS-only Scaleway A record.
9. Verify TLS, root OAuth metadata/JWKS/callback, login and logout, cookie scope,
   SSR/SEO/OG, same-origin API reads and writes, legacy native OAuth, service
   DID, extension/native API, Bunny, TAP, backups, and logs.
10. If stable, record evidence in `SCALEWAY_MIGRATION.md`; if not, restore root
    DNS to Netlify and recreate appview/Caddy with the Phase A env.
