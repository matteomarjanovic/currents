# Inference Server

FastAPI server that generates embeddings using [SigLIP2](https://huggingface.co/google/siglip2-base-patch16-naflex) (`google/siglip2-base-patch16-naflex`). Image and text embeddings share the same vector space, enabling multimodal retrieval.

Targets Apple Silicon (`mps`). Falls back to CPU if MPS is unavailable.

Requires Python 3.10+.

## Endpoints

| Method | Path           | Description                                                         |
|--------|----------------|---------------------------------------------------------------------|
| `POST` | `/embed/image` | Embed an image and return image metadata (`multipart/form-data`)    |
| `POST` | `/prepare/image` | Decode and resize an oversized image for upload (`multipart/form-data`) |
| `POST` | `/embed/text`  | Embed a query string (`{"text": "..."}`)                            |
| `POST` | `/reload-umap` | Re-read the UMAP model from disk (204 No Content)                   |
| `GET`  | `/health`      | Returns `{status, device, model, umap, queues, batches}`            |

`/embed/text` returns `{"embedding": [float × 768]}`.

`/embed/image` returns:

```json
{
	"embedding": [float x 768],
	"umap_embedding": [float x 50] | null,
	"width": 1234,
	"height": 987,
	"dominant_colors": [
		{"hex": "#aabbcc", "fraction": 0.42}
	]
}
```

`umap_embedding` is `null` when no UMAP model is loaded. The dominant-color palette and dimensions are returned from the same decoded image that feeds the embedding model.

`/prepare/image` accepts `multipart/form-data` with fields `file` and `max_bytes`, then returns the original image bytes when they already fit or a JPEG-transcoded payload when the image had to be reduced to satisfy the byte limit.

HEIC and HEIF inputs are supported through `pillow-heif`, in addition to the formats Pillow already decodes natively.

Text requests are batched automatically (up to 32 per batch, 20 ms collection window). Image requests are also batched automatically (up to 8 per batch, 20 ms collection window by default). Both modalities share a single model executor so only one batch runs on MPS at a time.

Both queues are bounded. When the server falls behind it returns HTTP 503 instead of allowing request latency to grow without bound.

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `TEXT_QUEUE_SIZE` | `256` | Maximum number of queued text requests |
| `IMAGE_QUEUE_SIZE` | `64` | Maximum number of queued image requests |
| `IMAGE_MAX_BATCH` | `8` | Maximum images per inference batch |
| `IMAGE_MAX_WAIT_SECS` | `0.020` | Image batch collection window in seconds |

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
