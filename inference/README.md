# Inference Server

FastAPI server that generates embeddings using [SigLIP2](https://huggingface.co/google/siglip2-base-patch16-naflex) (`google/siglip2-base-patch16-naflex`). Image and text embeddings share the same vector space, enabling multimodal retrieval.

Targets Apple Silicon (`mps`). Falls back to CPU if MPS is unavailable.

Requires Python 3.10+.

## Endpoints

| Method | Path           | Description                                                         |
|--------|----------------|---------------------------------------------------------------------|
| `POST` | `/embed/image` | Embed an image (`multipart/form-data`, field `file`)                |
| `POST` | `/embed/text`  | Embed a query string (`{"text": "..."}`)                            |
| `POST` | `/reload-umap` | Re-read the UMAP model from disk (204 No Content)                   |
| `GET`  | `/health`      | Returns `{status, device, model, umap}`                             |

`/embed/text` returns `{"embedding": [float × 768]}`.

`/embed/image` returns `{"embedding": [float × 768], "umap_embedding": [float × 50] | null}`. `umap_embedding` is `null` when no UMAP model is loaded.

Text requests are batched automatically (up to 32 per batch, 20 ms collection window) to maximize GPU throughput. Image requests run on a single-threaded executor to avoid memory contention.

## UMAP

On startup the server tries to load a UMAP model from `$MODELS_DIR/umap_model.joblib` (default `MODELS_DIR=./models`). When present, each image embed also returns its 50-dim UMAP projection. The `clustering` service trains/updates this file and calls `POST /reload-umap` afterwards so the server picks up the new model without restarting.

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
