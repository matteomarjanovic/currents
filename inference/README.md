# Inference Server

FastAPI server that generates embeddings using [SigLIP2](https://huggingface.co/google/siglip2-base-patch16-naflex) (`google/siglip2-base-patch16-naflex`). Image and text embeddings share the same vector space, enabling multimodal retrieval.

Targets Apple Silicon (`mps`). Falls back to CPU if MPS is unavailable.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/embed/image` | Embed an image (`multipart/form-data`, field `file`) |
| `POST` | `/embed/text` | Embed a query string (`{"text": "..."}`) |
| `GET` | `/health` | Returns model and device status |

Both embed endpoints return `{"embedding": [float, ...]}` — a 768-dimensional vector.

Text requests are batched automatically (up to 32 per batch, 20 ms collection window) to maximize GPU throughput. Image requests run on a single-threaded executor to avoid memory contention.

## Setup

```bash
python -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

## Running

```bash
uvicorn main:app --reload
```

Server listens on `http://localhost:8000` by default.
