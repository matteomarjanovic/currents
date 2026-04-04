# Currents

A calm, ad-free visual inspiration platform built on the [AT Protocol](https://atproto.com). Users save images into Collections.

## Services

| Directory | Language | Description |
|---|---|---|
| `appview/` | Go | AT Protocol AppView — HTTP server with OAuth, TAP listener, backed by PostgreSQL + pgvector |
| `inference/` | Python | FastAPI inference server — embeds images and text into a shared vector space (SigLIP2) |

The appview subscribes to a [TAP](https://github.com/bluesky-social/indigo/tree/main/cmd/tap) WebSocket at startup to index `is.currents.*` records from the AT Protocol firehose into local `collection` and `save` tables.

## Quick start

```bash
SESSION_SECRET=<random-string> docker compose up --build
```

- AppView: http://localhost:8080
- Inference: run separately (see `inference/`)

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
