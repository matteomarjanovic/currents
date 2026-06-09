# Moderation — Production Deployment

Architecture context lives in [`MODERATION.md`](MODERATION.md).

---

## Checklist

- [ ] 1. Generate a secp256k1 signing keypair
- [ ] 2. Provision the labeler atproto identity
- [ ] 3. Set `LABELER_*` env vars and restart the appview
- [ ] 4. Verify the DID document resolves
- [ ] 5. Bootstrap your first moderator
- [ ] 6. (Optional) Publish the `app.bsky.labeler.service` record for Bluesky discoverability
- [ ] 7. (When ready) Train + deploy the safety heads
- [ ] 8. (When ready) Run the moderation backfill

---

## 1. Generate a secp256k1 signing keypair

Install the atproto CLI:

```bash
go install github.com/bluesky-social/goat@latest
```

Generate the key:

```bash
goat key generate --type k256
# privateKey:  z3u2YGfZcEojNX7Bd...   ← goes in LABELER_SIGNING_KEY
# publicKey:   zQ3shok...
# didKey:      did:key:zQ3shok...
```

Save the `privateKey` value in your password manager — this is `LABELER_SIGNING_KEY`.

---

## 2. Provision the labeler atproto identity

**Option A — did:plc (recommended, no subdomain needed)**

The appview signs labels with the multibase key from step 1. For Bluesky to verify those signatures, that key's **public half** must live in the DID document under `#atproto_label`, and the DID must advertise an `AtprotoLabeler` service endpoint pointing at the appview. We do this with a manual PLC operation via `goat`.

> The Bluesky web app's "Become a labeler" button (when it appears) generates *its own* signing key — which wouldn't match the appview's. Don't use it. The steps below put *your* key in the DID doc instead.

1. Create a Bluesky account for moderation (e.g. handle `moderation.currents.is`, or a `*.bsky.social` handle). Note its `did:plc:...` — this is your `LABELER_DID`.

2. Log in with `goat`. PLC operations require your **main account password**, not an app password:

   ```bash
   goat account login -u moderation.currents.is -p '<main-account-password>'
   ```

3. Dump the current DID credentials as a starting point:

   ```bash
   goat account plc recommended > plc-op.json
   ```

4. Edit `plc-op.json`. Add the `atproto_label` verification method (the **`didKey`** value printed by `goat key generate` in step 1) and the `atproto_labeler` service. Leave the existing `atproto` key and `atproto_pds` service untouched:

   ```jsonc
   {
     "rotationKeys": ["did:key:..."],
     "alsoKnownAs": ["at://moderation.currents.is"],
     "verificationMethods": {
       "atproto": "did:key:zQ3sh...",          // existing — leave as-is
       "atproto_label": "did:key:zQ3shok..."   // ADD: the didKey from step 1
     },
     "services": {
       "atproto_pds": {                          // existing — leave as-is
         "type": "AtprotoPersonalDataServer",
         "endpoint": "https://<your-pds-host>"
       },
       "atproto_labeler": {                      // ADD this whole entry
         "type": "AtprotoLabeler",
         "endpoint": "https://<your-main-domain>"
       }
     }
   }
   ```

   The `atproto_labeler` endpoint is your **main appview domain** (e.g. `https://currents.is`) — the appview already serves `subscribeLabels` / `queryLabels` / `createReport` at the root.

5. Request a signing token (emailed to the account), then sign and submit:

   ```bash
   goat account plc request-token
   # check email for the token, then:
   goat account plc sign --token <TOKEN> plc-op.json > plc-signed.json
   goat account plc submit plc-signed.json
   ```

The appview skips serving a DID doc for `did:plc` identities — the PLC directory now resolves the credentials you just published.

**Option B — did:web:moderation.\<your-host\> (if you want the DID tied to your domain)**

1. In Cloudflare DNS, add a CNAME for `moderation.<your-host>` pointing at the same tunnel.
2. In the Cloudflare Tunnel dashboard, add `moderation.<your-host>` as a public hostname, routing to the same nginx backend.
3. Set `LABELER_DID=did:web:moderation.<your-host>`. The appview serves the DID doc at `https://moderation.<your-host>/.well-known/did.json` automatically once the env var is set.

---

## 3. Set env vars and restart

Add to `docker-compose.mac-mini.yml` (already done — see `appview.environment`):

```
LABELER_DID=did:plc:abc...
LABELER_SIGNING_KEY=z3u2YGfZcEojNX7Bd...
```

On startup the appview logs:

```
labeler enabled did=did:plc:... host= publicKey=z...
```

(`host=` is populated only for `did:web`.)

---

## 4. Verify the DID document resolves

```bash
# did:plc
curl https://plc.directory/did:plc:<your-id> | jq

# did:web
curl https://moderation.<your-host>/.well-known/did.json | jq
```

Both should include a `verificationMethod[]` entry with id ending `#atproto_label` and a `service[]` entry of type `AtprotoLabeler` whose `serviceEndpoint` is your appview domain. The `publicKeyMultibase` there must match the `publicKey` the appview logged at startup (step 3) — if they differ, you put the wrong key in the PLC operation.

---

## 5. Bootstrap your first moderator

```sql
INSERT INTO moderator (did, role) VALUES ('did:plc:your-did-here', 'admin');
```

That DID can now visit `/admin/queue`. To revoke:

```sql
UPDATE moderator SET disabled_at = now() WHERE did = 'did:plc:...';
```

---

## 6. (Optional) Publish the labeler.service record

This makes the labeler subscribable from Bluesky's UI. Skip until you're ready to go public.

Save as `labeler.service.json`:

```json
{
  "$type": "app.bsky.labeler.service",
  "createdAt": "2026-05-17T00:00:00Z",
  "policies": {
    "labelValues": [
      "porn", "sexual", "graphic-media", "nudity",
      "currents-nsfw-suspected",
      "currents-violence-suspected",
      "currents-ai-generated",
      "currents-ai-generated-suspected",
      "!hide"
    ],
    "labelValueDefinitions": [
      {
        "identifier": "currents-nsfw-suspected",
        "blurs": "media", "severity": "alert", "defaultSetting": "warn", "adultOnly": true,
        "locales": [{"lang": "en", "name": "NSFW (under review)",
          "description": "Automatically flagged as possibly explicit; awaiting human review."}]
      },
      {
        "identifier": "currents-violence-suspected",
        "blurs": "media", "severity": "alert", "defaultSetting": "warn", "adultOnly": false,
        "locales": [{"lang": "en", "name": "Graphic violence (under review)",
          "description": "Automatically flagged as possibly violent; awaiting human review."}]
      },
      {
        "identifier": "currents-ai-generated",
        "blurs": "none", "severity": "inform", "defaultSetting": "ignore", "adultOnly": false,
        "locales": [{"lang": "en", "name": "AI-generated",
          "description": "Detected as likely AI-generated imagery."}]
      },
      {
        "identifier": "currents-ai-generated-suspected",
        "blurs": "none", "severity": "inform", "defaultSetting": "ignore", "adultOnly": false,
        "locales": [{"lang": "en", "name": "AI-generated (under review)",
          "description": "Automatically flagged as possibly AI-generated; awaiting author or moderator confirmation."}]
      }
    ]
  },
  "subjectTypes": ["record"],
  "subjectCollections": ["is.currents.feed.save"],
  "reasonTypes": [
    "com.atproto.moderation.defs#reasonSexual",
    "com.atproto.moderation.defs#reasonViolation",
    "com.atproto.moderation.defs#reasonOther"
  ]
}
```

Publish with `goat` (logged in as the labeler account):

```bash
goat account login --handle moderation.<your-host> --password "<app-password>"
goat record create --collection app.bsky.labeler.service --rkey self --record labeler.service.json
```

---

## 7. Train + deploy the safety heads

The training notebooks are in [`moderation/`](moderation/). Each exports an ONNX file with input `embedding (float32[1,768])` and output `logits (float32[1,1])`.

Once you have `*.onnx` files, set on the inference server before restart:

```bash
NSFW_HEAD_ONNX=/srv/models/nsfw_head.onnx
VIOLENCE_HEAD_ONNX=/srv/models/violence_head.onnx
AIGEN_HEAD_ONNX=/srv/models/aigen_head.onnx
```

`GET /health` shows loaded heads:

```json
{ "safety_heads": { "nsfw": true, "violence": false, "ai_generated": false } }
```

Heads deploy independently — unloaded heads contribute `0.0`, so no flag fires for them.

---

## 8. Run the moderation backfill

After heads are deployed, score existing saves:

```bash
# Preview — safe, no writes
appview backfill-moderation --dry-run

# Staged run: eyeball the admin queue before continuing
appview backfill-moderation --limit 1000

# Full run
appview backfill-moderation
```

Cancel-safe (Ctrl+C honored between batches). Restarting is idempotent — already-classified blobs are skipped.

---

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `labeler disabled (no LABELER_SIGNING_KEY)` | env var missing or empty; check docker-compose and restart |
| `labeler DID required when signing key is set` | `LABELER_DID` not set alongside `LABELER_SIGNING_KEY` |
| `subscribeLabels` WebSocket immediately disconnects | nginx missing `Upgrade`/`Connection` headers — see `nginx/default.conf` |
| `/.well-known/did.json` on moderation subdomain returns the appview DID doc | `Host` header not reaching appview; check nginx `proxy_set_header Host $host` |
| Admin page redirects to `/` even when signed in | session DID not in `moderator` table, or `disabled_at` is set |
| `safety_scores: null` in every `/embed/image` response | no head env vars on inference server; check `GET /health → safety_heads` |
| `backfill-moderation` errors `503 no safety heads loaded` | inference server has no heads loaded; see step 7 |
