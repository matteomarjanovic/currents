# PDS image-fetch security and practical checks

PDS endpoints come from users' DID documents and cached account data. Treat
both as untrusted. The HTTP transports for PDS blobs and identity lookup now
check the resolved IP at connection time, including after redirects. They do
not use environment proxies, which could resolve a target outside that check.
Loopback, private, link-local (including cloud metadata), multicast, and
unspecified addresses are blocked. Redirects are capped at four hops.

Blob downloads are limited to **20,000,000 bytes**, matching the image lexicon,
including when Content-Length is missing or misleading. The proxy identifies
raster formats from bytes, ignores the upstream Content-Type, rejects HTML/SVG
and unknown content, and sends `X-Content-Type-Options: nosniff` plus
`Content-Security-Policy: default-src 'none'; sandbox`. JPEG, PNG, GIF, WebP,
BMP, ICO, TIFF, AVIF, and HEIC/HEIF signatures are accepted. HEIC/HEIF still goes
through inference conversion; its returned bytes are checked too. Signature
checks are not a full decoder: malformed raster files can still fail image
inference/rendering. Decoder dependency updates remain a separate task.

The address policy also applies to cached PDS endpoints, DID-resolution
fallbacks, and public getRecord calls using the shared PDS HTTP client. Private
PDS servers are deliberately unreachable; this includes private development
PDS servers. The configured internal inference client is separate and keeps
working over the private network.

## Automated regressions

From the repository root, without any external services:

```sh
cd appview
go test -run 'Test(PDSFetch|BlobImageType|IdentityHTTP|IsDisallowedIP)' -v .
```

The tests check actual connection refusal before a local fixture server is
contacted, localhost DNS resolution, IPv4/IPv6 and mapped IPv4, redirect hops
and loops, permitted public redirects, proxy bypass prevention, query escaping,
byte limits and response-body closure, forged MIME types, and supported raster
signatures. They make no cloud-metadata requests.

To run the real image-proxy handler, enrichment lifecycle, and full Go suite,
use the isolated local PostgreSQL test container (port 55432 in this workspace):

```sh
TEST_DATABASE_URL='postgres://appview:appview@localhost:55432/appview_enrichment_test?sslmode=disable' \
  go test ./...
```

The database suite **resets its test schema**. Use only a disposable database
whose name ends in `_test`. With no TEST_DATABASE_URL, DB tests are skipped.
Handler regressions verify successful image responses, MIME correction, HTML/SVG
rejection, HEIC conversion, unsafe conversion output rejection, security
headers, public image CORS, and immutable caching only on successful responses.

## Normal use in the browser

After starting the changed appview, or deploying it:

1. Open a collection and a profile: images and avatars should load.
2. Save a JPEG/PNG, then resave somebody else's image into your collection.
   Also try an image hosted on a custom **public** PDS.
3. Open an existing HEIC/HEIF image: the converted image should still load.
   Open an AVIF too if one is available.
4. In browser Network tools, inspect the image's `/img/{did}/{cid}` response.
   Expect 200, the correct raster Content-Type, `nosniff`, the sandbox CSP,
   public CORS (`*`, without credentials), and immutable caching.

For a direct header check, take a real DID and blob CID from an image URL and
request the appview origin rather than the image CDN:

```sh
curl -sS -D /tmp/currents-image-headers.txt \
  -o /tmp/currents-image.bin \
  'https://currents.is/img/REAL_DID/REAL_BLOB_CID'
cat /tmp/currents-image-headers.txt
file /tmp/currents-image.bin
```

Use GET so you can check the body as well as the headers. At deployment,
purge existing `/img/` entries in the Bunny image cache so old responses do not
hide the new MIME checks or headers. Already cached responses are not rerun
through the application by a deployment alone.

## Optional end-to-end rejection checks with an owned public fixture

These checks require a public HTTPS domain/server you control. They are not
necessary for running the automated regressions above.

Serve this at `https://FIXTURE_HOST/.well-known/did.json`:

```json
{
  "id": "did:web:FIXTURE_HOST",
  "service": [{
    "id": "#atproto_pds",
    "type": "AtprotoPersonalDataServer",
    "serviceEndpoint": "https://FIXTURE_HOST"
  }]
}
```

Replace FIXTURE_HOST everywhere with your real hostname. On that same server,
make `/xrpc/com.atproto.sync.getBlob` choose a response using the `cid` query
parameter. No login, indexed saves, or real PDS records are needed for this
fixture. Then request:

```sh
curl -sS -D - 'http://localhost:8080/img/did:web:FIXTURE_HOST/png' -o /tmp/proxy-result
```

Replace localhost:8080 with your running changed appview as needed.

| Fixture response | Expected proxy result |
|---|---|
| `cid=png`: real PNG bytes, even with `Content-Type: text/html` | 200, `image/png`, same image bytes |
| `cid=html`: HTML with `Content-Type: image/png` | 502, short error, no upstream HTML |
| `cid=svg`: an SVG document | 502, no SVG document |
| `cid=large`: 20,000,001 bytes, with or without Content-Length | 502; app stops reading at the limit plus one byte |
| `cid=redirect`: 302 to a second owned public host serving a PNG | 200 with PNG |
| `cid=private`: 302 to an owned private/loopback fixture listener | 502; private fixture access log stays empty |

Use an ordinary private fixture listener for the last case, never a real
metadata service. To test the DID-resolution boundary itself, point an owned
public hostname at loopback/private IP: lookup should fail before any HTTP
request or TLS handshake reaches that listener. Public DID lookups can be
cached; use a fresh owned test hostname when changing the DID document.
