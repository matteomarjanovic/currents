# Currents

A calm, ad-free visual inspiration platform built on the [AT Protocol](https://atproto.com). Users save images into Collections.

## Services

| Directory | Language | Description |
|---|---|---|
| `appview/` | Go | AT Protocol AppView — HTTP server with OAuth, TAP listener, backed by PostgreSQL + pgvector |
| `inference/` | Python | FastAPI inference server — embeds images and text into a shared vector space (SigLIP2) |

The appview subscribes to a [TAP](https://github.com/bluesky-social/indigo/tree/main/cmd/tap) WebSocket at startup to index `is.currents.*` records from the AT Protocol firehose into local `collection` and `save` tables. Saves are always written to the user's PDS first (via the browser extension or the appview's `POST /resave` endpoint for resaves), then indexed asynchronously by the TAP listener — the appview DB is never written to directly by HTTP handlers for user content.

## Quick start

```bash
SESSION_SECRET=<random-string> docker compose up --build
```

- AppView: http://localhost:8080
- Inference: run separately (see `inference/`)

## Dev TAP stack

For local TAP debugging, use the dev compose file. It tracks only the handles in
`TAP_DEV_HANDLES`, seeds TAP via `/repos/add`, and starts the full local stack,
including `clustering`.

```bash
TAP_DEV_HANDLES="matteomarjanovic.com,zeroassembly.site" \
docker compose -f docker-compose.dev.yml up -d --build
```

Add more tracked repos by appending more comma-separated handles to
`TAP_DEV_HANDLES`.

To wipe the dev stack completely:

```bash
docker compose -f docker-compose.dev.yml down -v
```

## Inference server

```bash
cd inference
pip install -r requirements.txt
uvicorn main:app --reload
```

Endpoints: `POST /embed/image`, `POST /embed/text`, `POST /analyze/image`, `GET /health`

`/analyze/image` returns an embedding, dominant-color palette, and image dimensions in one call — used by the appview when indexing new saves.

## More

See `appview/README.md` for full AppView configuration and OAuth docs.
