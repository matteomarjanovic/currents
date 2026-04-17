# main.py
import asyncio
import io
import os
import threading
from contextlib import asynccontextmanager
from concurrent.futures import ThreadPoolExecutor

import joblib
import numpy as np
import torch
from fastapi import FastAPI, File, UploadFile, HTTPException
from PIL import Image
from pydantic import BaseModel
from transformers import AutoModel, AutoProcessor

# ── Config ────────────────────────────────────────────────────────────────────

CHECKPOINT    = "google/siglip2-base-patch16-naflex"
DEVICE        = "mps" if torch.backends.mps.is_available() else "cpu"
DTYPE         = torch.bfloat16
MAX_PATCHES   = 256
TEXT_MIN_BATCH     = 8
TEXT_MAX_BATCH     = 32
TEXT_MAX_WAIT_SECS = 0.020
TEXT_QUEUE_SIZE    = int(os.environ.get("TEXT_QUEUE_SIZE", "256"))
IMAGE_MAX_BATCH    = int(os.environ.get("IMAGE_MAX_BATCH", "8"))
IMAGE_MAX_WAIT_SECS = float(os.environ.get("IMAGE_MAX_WAIT_SECS", "0.020"))
IMAGE_QUEUE_SIZE    = int(os.environ.get("IMAGE_QUEUE_SIZE", "64"))

MODELS_DIR = os.environ.get("MODELS_DIR", "./models")
UMAP_PATH  = os.path.join(MODELS_DIR, "umap_model.joblib")

# ── State ─────────────────────────────────────────────────────────────────────

model     = None
processor = None
executor  = ThreadPoolExecutor(max_workers=1)

text_queue: asyncio.Queue = asyncio.Queue(maxsize=TEXT_QUEUE_SIZE)
image_queue: asyncio.Queue = asyncio.Queue(maxsize=IMAGE_QUEUE_SIZE)
worker_tasks: list[asyncio.Task] = []

_umap_model:       object = None
_umap_model_mtime: float  = None
_umap_lock = threading.Lock()

# ── UMAP loading ──────────────────────────────────────────────────────────────

def _try_load_umap():
    global _umap_model, _umap_model_mtime
    if not os.path.exists(UMAP_PATH):
        return
    try:
        mtime = os.path.getmtime(UMAP_PATH)
        with _umap_lock:
            if _umap_model_mtime is not None and mtime <= _umap_model_mtime:
                return
            _umap_model = joblib.load(UMAP_PATH)
            _umap_model_mtime = mtime
        print(f"UMAP model loaded from {UMAP_PATH}")
    except Exception as e:
        print(f"Failed to load UMAP model: {e}")

# ── Inference ─────────────────────────────────────────────────────────────────

def _embed_images(images: list[Image.Image]) -> list[tuple[list[float], list[float] | None]]:
    inputs = processor(
        images=images,
        max_num_patches=MAX_PATCHES,
        return_tensors="pt",
    ).to(DEVICE)
    with torch.no_grad():
        features = model.get_image_features(**inputs)
    if hasattr(features, "pooler_output"):
        features = features.pooler_output
    embeddings = features.cpu().float().numpy()

    with _umap_lock:
        local_umap = _umap_model
    umap_embeddings = None
    if local_umap is not None:
        try:
            umap_embeddings = local_umap.transform(embeddings.astype(np.float32)).tolist()
        except Exception as e:
            print(f"UMAP transform failed: {e}")

    results = []
    for idx, embedding in enumerate(embeddings.tolist()):
        umap_embedding = None if umap_embeddings is None else umap_embeddings[idx]
        results.append((embedding, umap_embedding))
    return results


def _embed_texts(texts: list[str]) -> list[list[float]]:
    inputs = processor(
        text=texts,
        max_length=64,
        padding="max_length",
        truncation=True,
        return_tensors="pt",
    ).to(DEVICE)
    with torch.no_grad():
        features = model.get_text_features(**inputs)
    if hasattr(features, "pooler_output"):
        features = features.pooler_output
    return features.cpu().float().numpy().tolist()

# ── Text batch worker ─────────────────────────────────────────────────────────

async def _text_batch_worker():
    loop = asyncio.get_running_loop()
    while True:
        first_text, first_future = await text_queue.get()
        batch   = [first_text]
        futures = [first_future]

        deadline = loop.time() + TEXT_MAX_WAIT_SECS
        try:
            while len(batch) < TEXT_MAX_BATCH:
                timeout = deadline - loop.time()
                if timeout <= 0:
                    break
                text, fut = await asyncio.wait_for(text_queue.get(), timeout=timeout)
                batch.append(text)
                futures.append(fut)
                if len(batch) >= TEXT_MIN_BATCH:
                    break
        except asyncio.TimeoutError:
            pass

        try:
            results = await loop.run_in_executor(executor, _embed_texts, batch)
            for fut, result in zip(futures, results):
                if not fut.done():
                    fut.set_result(result)
        except Exception as exc:
            for fut in futures:
                if not fut.done():
                    fut.set_exception(exc)


async def _image_batch_worker():
    loop = asyncio.get_running_loop()
    while True:
        first_image, first_future = await image_queue.get()
        batch = [first_image]
        futures = [first_future]

        deadline = loop.time() + IMAGE_MAX_WAIT_SECS
        try:
            while len(batch) < IMAGE_MAX_BATCH:
                timeout = deadline - loop.time()
                if timeout <= 0:
                    break
                image, fut = await asyncio.wait_for(image_queue.get(), timeout=timeout)
                batch.append(image)
                futures.append(fut)
        except asyncio.TimeoutError:
            pass

        try:
            results = await loop.run_in_executor(executor, _embed_images, batch)
            for fut, result in zip(futures, results):
                if not fut.done():
                    fut.set_result(result)
        except Exception as exc:
            for fut in futures:
                if not fut.done():
                    fut.set_exception(exc)

# ── Lifespan ──────────────────────────────────────────────────────────────────

@asynccontextmanager
async def lifespan(app: FastAPI):
    global model, processor

    print(f"Loading {CHECKPOINT} on {DEVICE}…")
    processor = AutoProcessor.from_pretrained(CHECKPOINT)
    model = (
        AutoModel.from_pretrained(CHECKPOINT, torch_dtype=DTYPE)
        .to(DEVICE)
        .eval()
    )
    print("Model ready.")

    _try_load_umap()

    worker_tasks.clear()
    worker_tasks.extend([
        asyncio.create_task(_text_batch_worker()),
        asyncio.create_task(_image_batch_worker()),
    ])

    yield

    for task in worker_tasks:
        task.cancel()
    if worker_tasks:
        await asyncio.gather(*worker_tasks, return_exceptions=True)
    worker_tasks.clear()

    executor.shutdown(wait=False)

# ── App ───────────────────────────────────────────────────────────────────────

app = FastAPI(lifespan=lifespan)

# ── Routes ────────────────────────────────────────────────────────────────────

class TextRequest(BaseModel):
    text: str

class EmbeddingResponse(BaseModel):
    embedding: list[float]

class ImageEmbeddingResponse(BaseModel):
    embedding: list[float]
    umap_embedding: list[float] | None = None

@app.post("/embed/image", response_model=ImageEmbeddingResponse)
async def embed_image(file: UploadFile = File(...)):
    if not file.content_type or not file.content_type.startswith("image/"):
        raise HTTPException(status_code=400, detail="File must be an image")

    raw = await file.read()
    try:
        image = Image.open(io.BytesIO(raw)).convert("RGB")
    except Exception:
        raise HTTPException(status_code=400, detail="Could not decode image")

    loop = asyncio.get_running_loop()
    future = loop.create_future()
    try:
        image_queue.put_nowait((image, future))
    except asyncio.QueueFull:
        raise HTTPException(status_code=503, detail="Image inference queue is full")

    embedding, umap_embedding = await future
    return ImageEmbeddingResponse(embedding=embedding, umap_embedding=umap_embedding)


@app.post("/embed/text", response_model=EmbeddingResponse)
async def embed_text(req: TextRequest):
    loop   = asyncio.get_running_loop()
    future = loop.create_future()
    try:
        text_queue.put_nowait((req.text, future))
    except asyncio.QueueFull:
        raise HTTPException(status_code=503, detail="Text inference queue is full")
    embedding = await future
    return EmbeddingResponse(embedding=embedding)


@app.post("/reload-umap", status_code=204)
async def reload_umap():
    _try_load_umap()


@app.get("/health")
async def health():
    with _umap_lock:
        umap_loaded = _umap_model is not None
    return {
        "status": "ok",
        "device": DEVICE,
        "model": CHECKPOINT,
        "umap": umap_loaded,
        "queues": {
            "text": {"pending": text_queue.qsize(), "max": TEXT_QUEUE_SIZE},
            "image": {"pending": image_queue.qsize(), "max": IMAGE_QUEUE_SIZE},
        },
        "batches": {
            "text": {"min": TEXT_MIN_BATCH, "max": TEXT_MAX_BATCH, "wait_ms": int(TEXT_MAX_WAIT_SECS * 1000)},
            "image": {"max": IMAGE_MAX_BATCH, "wait_ms": int(IMAGE_MAX_WAIT_SECS * 1000)},
        },
    }
