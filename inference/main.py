# main.py
import asyncio
import io
import math
import os
import threading
from contextlib import asynccontextmanager
from concurrent.futures import ThreadPoolExecutor

import joblib
import numpy as np
import onnxruntime as ort
import torch
from fastapi import FastAPI, File, Form, HTTPException, Response, UploadFile
from PIL import Image
from pillow_heif import register_heif_opener
from pydantic import BaseModel
from transformers import AutoModel, AutoProcessor

register_heif_opener(thumbnails=False)

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
PREPARE_MAX_STEPS   = 10
PREPARE_SCALE       = 0.85
PREPARE_QUALITY     = 85
PALETTE_SIZE          = 5      # colors in the output palette
PALETTE_K             = 12     # over-clustering k, merged/filtered down to PALETTE_SIZE
PALETTE_ITERS         = 12
PALETTE_THUMB_SIZE    = 128
PALETTE_MERGE_DE      = 10.0   # ΔE76 below which clusters merge
PALETTE_MIN_FRACTION  = 0.004  # noise floor (~66 px at 128×128)
PALETTE_CHROMA_WEIGHT = 2.0    # saturation boost when selecting palette colors

MODELS_DIR = os.environ.get("MODELS_DIR", "./models")
UMAP_PATH  = os.path.join(MODELS_DIR, "umap_model.joblib")
RESAMPLE_NEAREST = getattr(Image, "Resampling", Image).NEAREST

# Moderation heads — optional ONNX classifiers loaded at startup. Each runs on
# the L2-normalized 768-d SigLIP2 pooler output and returns a single logit.
# Missing or unset paths leave the head None; downstream emits 0.0 for that axis.
SAFETY_HEAD_ENV = {
    "nsfw":         "NSFW_HEAD_ONNX",
    "violence":     "VIOLENCE_HEAD_ONNX",
    "ai_generated": "AIGEN_HEAD_ONNX",
}
# Feed junk head (suitable vs junk for the global discovery feed) — same ONNX
# contract as the safety heads, trained in moderation/04_feed_junk_head.ipynb.
JUNK_HEAD_ENV = "FEED_JUNK_HEAD_ONNX"
EMBEDDING_DIM = 768

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

_safety_heads: dict[str, ort.InferenceSession | None] = {axis: None for axis in SAFETY_HEAD_ENV}
_junk_head: ort.InferenceSession | None = None

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

# ── Safety heads ──────────────────────────────────────────────────────────────

def _load_safety_heads():
    """Load each ONNX head. Path resolution order:
    1. Explicit env var (e.g. NSFW_HEAD_ONNX)
    2. $MODELS_DIR/{axis}_head.onnx   (convention-based auto-detect)
    Missing or unset paths leave that axis disabled (emits 0.0 downstream)."""
    for axis, env_var in SAFETY_HEAD_ENV.items():
        path = os.environ.get(env_var) or os.path.join(MODELS_DIR, f"{axis}_head.onnx")
        if not os.path.exists(path):
            continue
        try:
            _safety_heads[axis] = ort.InferenceSession(path, providers=["CPUExecutionProvider"])
            print(f"{axis} head loaded from {path}")
        except Exception as exc:
            print(f"failed to load {axis} head from {path}: {exc}")


def _load_junk_head():
    """Load the feed junk head. Path resolution order:
    1. Explicit env var (FEED_JUNK_HEAD_ONNX)
    2. $MODELS_DIR/feed_junk_head.onnx   (convention-based auto-detect)
    A missing path leaves the head disabled (junk_score stays None downstream)."""
    global _junk_head
    path = os.environ.get(JUNK_HEAD_ENV) or os.path.join(MODELS_DIR, "feed_junk_head.onnx")
    if not os.path.exists(path):
        return
    try:
        _junk_head = ort.InferenceSession(path, providers=["CPUExecutionProvider"])
        print(f"feed junk head loaded from {path}")
    except Exception as exc:
        print(f"failed to load feed junk head from {path}: {exc}")


def _any_safety_head_loaded() -> bool:
    return any(sess is not None for sess in _safety_heads.values())


def _l2_normalize(x: np.ndarray) -> np.ndarray:
    """Row-wise L2 normalization. Treats zero-norm rows as identity to avoid NaN."""
    norms = np.linalg.norm(x, axis=1, keepdims=True)
    norms = np.where(norms == 0, 1.0, norms)
    return x / norms


def _score_junk(embeddings_norm: np.ndarray) -> list[float] | None:
    """Junk probability (1 = unsuitable for the global feed) per row of a batch
    of L2-normalized 768-d embeddings, or None when no head is loaded."""
    if _junk_head is None:
        return None
    logits = _junk_head.run(None, {"embedding": embeddings_norm})[0]
    # sigmoid in float64 to avoid overflow at extreme logits
    return (1.0 / (1.0 + np.exp(-logits.astype(np.float64)))).ravel().tolist()


def _classify_safety(embeddings_norm: np.ndarray) -> list[dict[str, float]]:
    """Run all loaded heads on a batch of L2-normalized 768-d embeddings.
    Returns one dict per row; unloaded axes contribute 0.0 so downstream
    threshold logic treats them as 'no signal'."""
    n = embeddings_norm.shape[0]
    head_probs: dict[str, np.ndarray] = {}
    for axis, sess in _safety_heads.items():
        if sess is None:
            continue
        logits = sess.run(None, {"embedding": embeddings_norm})[0]
        # sigmoid in float64 to avoid overflow at extreme logits
        head_probs[axis] = (1.0 / (1.0 + np.exp(-logits.astype(np.float64)))).ravel()
    return [
        {axis: float(head_probs[axis][i]) if axis in head_probs else 0.0 for axis in SAFETY_HEAD_ENV}
        for i in range(n)
    ]


# ── Inference ─────────────────────────────────────────────────────────────────

def _embed_images(images: list[Image.Image]) -> list[dict]:
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

    # Classification heads are tiny MLPs (CPU); cost is negligible per batch
    # but they only run when loaded. Heads expect L2-normalized inputs (per
    # training in moderation/*.ipynb); the response embedding itself remains
    # un-normalized to preserve pgvector compatibility with rows written
    # before classification existed.
    safety_scores = None
    junk_scores = None
    if _any_safety_head_loaded() or _junk_head is not None:
        embeddings_norm = _l2_normalize(embeddings.astype(np.float32))
        if _any_safety_head_loaded():
            try:
                safety_scores = _classify_safety(embeddings_norm)
            except Exception as e:
                print(f"safety classification failed: {e}")
        try:
            junk_scores = _score_junk(embeddings_norm)
        except Exception as e:
            print(f"junk scoring failed: {e}")

    results = []
    for idx, embedding in enumerate(embeddings.tolist()):
        results.append({
            "embedding": embedding,
            "umap_embedding": None if umap_embeddings is None else umap_embeddings[idx],
            "safety_scores": None if safety_scores is None else safety_scores[idx],
            "junk_score": None if junk_scores is None else junk_scores[idx],
        })
    return results


def _embed_texts(texts: list[str]) -> list[list[float]]:
    # SigLIP text encoder is case-sensitive and trained on lowercased captions;
    # lowercasing keeps queries on-distribution (e.g. "Home" ≈ "home").
    texts = [t.lower() for t in texts]
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


def _fraction(value: float) -> float:
    return math.floor(value * 10000 + 0.5) / 10000


def _clamp_channel(value: float) -> int:
    if value < 0:
        return 0
    if value > 255:
        return 255
    return int(value)


def _srgb_to_lab(rgb: np.ndarray) -> np.ndarray:
    c = rgb / 255.0
    c = np.where(c <= 0.04045, c / 12.92, ((c + 0.055) / 1.055) ** 2.4)
    xyz = c @ np.array([
        [0.4124564, 0.2126729, 0.0193339],
        [0.3575761, 0.7151522, 0.1191920],
        [0.1804375, 0.0721750, 0.9503041],
    ])
    xyz /= np.array([0.95047, 1.0, 1.08883])  # D65 white point
    f = np.where(xyz > 0.008856, np.cbrt(xyz), xyz * 7.787 + 16.0 / 116.0)
    lab = np.empty_like(xyz)
    lab[:, 0] = 116.0 * f[:, 1] - 16.0
    lab[:, 1] = 500.0 * (f[:, 0] - f[:, 1])
    lab[:, 2] = 200.0 * (f[:, 1] - f[:, 2])
    return lab


def _kmeans(pixels: np.ndarray, k: int, iters: int) -> np.ndarray:
    step = max(1, len(pixels) // k)
    rng = np.random.default_rng()
    centroids = np.empty((k, 3), dtype=np.float64)
    for idx in range(k):
        start = min(idx * step, len(pixels) - 1)
        end = min(start + step, len(pixels))
        if end <= start:
            end = min(start + 1, len(pixels))
        centroids[idx] = pixels[rng.integers(start, end)]

    for _ in range(iters):
        distances = np.sum((pixels[:, None, :] - centroids[None, :, :]) ** 2, axis=2)
        assignments = np.argmin(distances, axis=1)

        for idx in range(k):
            members = pixels[assignments == idx]
            if len(members) == 0:
                centroids[idx] = pixels[rng.integers(0, len(pixels))]
                continue
            centroids[idx] = members.mean(axis=0)

    return centroids


def _dominant_colors(image: Image.Image) -> list[dict[str, float | str]]:
    # NEAREST keeps exact pixel colors, so small accents survive the resize.
    thumb = image.resize((PALETTE_THUMB_SIZE, PALETTE_THUMB_SIZE), RESAMPLE_NEAREST)
    pixels_rgb = np.asarray(thumb, dtype=np.float64).reshape(-1, 3)
    pixels_lab = _srgb_to_lab(pixels_rgb)

    centroids = _kmeans(pixels_lab, PALETTE_K, PALETTE_ITERS)
    distances = np.sum((pixels_lab[:, None, :] - centroids[None, :, :]) ** 2, axis=2)
    assignments = np.argmin(distances, axis=1)

    # The hex output is the mean RGB of each cluster's actual member pixels,
    # so no inverse Lab→sRGB transform is needed.
    clusters = []
    for idx in range(len(centroids)):
        members = assignments == idx
        count = int(members.sum())
        if count == 0:
            continue
        clusters.append({
            "count": count,
            "lab": pixels_lab[members].mean(axis=0),
            "rgb": pixels_rgb[members].mean(axis=0),
        })

    # Merge perceptually-close clusters, largest first; every survivor ends up
    # at least PALETTE_MERGE_DE from the others.
    clusters.sort(key=lambda c: c["count"], reverse=True)
    merged = []
    for cluster in clusters:
        for kept in merged:
            if np.linalg.norm(cluster["lab"] - kept["lab"]) < PALETTE_MERGE_DE:
                total = kept["count"] + cluster["count"]
                kept["lab"] = (kept["lab"] * kept["count"] + cluster["lab"] * cluster["count"]) / total
                kept["rgb"] = (kept["rgb"] * kept["count"] + cluster["rgb"] * cluster["count"]) / total
                kept["count"] = total
                break
        else:
            merged.append(cluster)

    # sqrt compresses area dominance and the chroma boost keeps small
    # saturated accents ahead of yet another shade of the background.
    total_pixels = float(len(pixels_lab))
    candidates = [c for c in merged if c["count"] / total_pixels >= PALETTE_MIN_FRACTION]
    candidates.sort(
        key=lambda c: math.sqrt(c["count"] / total_pixels)
        * (1.0 + PALETTE_CHROMA_WEIGHT * math.hypot(c["lab"][1], c["lab"][2]) / 100.0),
        reverse=True,
    )

    colors = []
    for cluster in candidates[:PALETTE_SIZE]:
        colors.append({
            "hex": "#{:02x}{:02x}{:02x}".format(
                _clamp_channel(cluster["rgb"][0]),
                _clamp_channel(cluster["rgb"][1]),
                _clamp_channel(cluster["rgb"][2]),
            ),
            "fraction": _fraction(cluster["count"] / total_pixels),
        })

    colors.sort(key=lambda color: color["fraction"], reverse=True)
    return colors


def _decode_image(raw: bytes) -> tuple[Image.Image, str]:
    try:
        opened = Image.open(io.BytesIO(raw))
        source_mime = Image.MIME.get(opened.format or "", "application/octet-stream")
        return opened.convert("RGB"), source_mime
    except Exception as exc:
        raise HTTPException(status_code=400, detail="Could not decode image") from exc


def _prepare_image_bytes(image: Image.Image, max_bytes: int) -> bytes:
    if max_bytes <= 0:
        raise ValueError("max_bytes must be positive")

    current = image
    for _ in range(PREPARE_MAX_STEPS):
        bounds = current.size
        width = max(1, int(bounds[0] * PREPARE_SCALE))
        height = max(1, int(bounds[1] * PREPARE_SCALE))
        current = current.resize((width, height), Image.BILINEAR)

        out = io.BytesIO()
        current.save(out, format="JPEG", quality=PREPARE_QUALITY)
        prepared = out.getvalue()
        if len(prepared) <= max_bytes:
            return prepared

    raise ValueError("could not shrink image below target size")

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
    _load_safety_heads()
    _load_junk_head()

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


class DominantColor(BaseModel):
    hex: str
    fraction: float

class SafetyScores(BaseModel):
    nsfw: float
    violence: float
    ai_generated: float

class ImageEmbeddingResponse(BaseModel):
    embedding: list[float]
    umap_embedding: list[float] | None = None
    width: int
    height: int
    dominant_colors: list[DominantColor]
    safety_scores: SafetyScores | None = None
    junk_score: float | None = None

@app.post("/embed/image", response_model=ImageEmbeddingResponse)
async def embed_image(file: UploadFile = File(...)):
    raw = await file.read()
    image, _ = _decode_image(raw)
    width, height = image.size
    dominant_colors = _dominant_colors(image)

    loop = asyncio.get_running_loop()
    future = loop.create_future()
    try:
        image_queue.put_nowait((image, future))
    except asyncio.QueueFull:
        raise HTTPException(status_code=503, detail="Image inference queue is full")

    result = await future
    scores = result["safety_scores"]
    return ImageEmbeddingResponse(
        embedding=result["embedding"],
        umap_embedding=result["umap_embedding"],
        width=width,
        height=height,
        dominant_colors=dominant_colors,
        safety_scores=SafetyScores(**scores) if scores is not None else None,
        junk_score=result["junk_score"],
    )


class PaletteResponse(BaseModel):
    dominant_colors: list[DominantColor]


def _palette_from_bytes(raw: bytes) -> list[dict[str, float | str]]:
    """Decode and extract in one executor hop, so neither half runs on the event
    loop — a full-resolution PIL decode is ~45 ms of blocking that would
    serialize concurrent palette requests and stall live /embed/image traffic.
    """
    image, _ = _decode_image(raw)
    return _dominant_colors(image)


@app.post("/palette", response_model=PaletteResponse)
async def palette(file: UploadFile = File(...)):
    raw = await file.read()
    loop = asyncio.get_running_loop()
    dominant_colors = await loop.run_in_executor(None, _palette_from_bytes, raw)
    return PaletteResponse(dominant_colors=dominant_colors)


def _encode_jpeg(image: Image.Image, quality: int) -> bytes:
    out = io.BytesIO()
    image.save(out, format="JPEG", quality=quality)
    return out.getvalue()


@app.post("/transcode/image")
async def transcode_image(file: UploadFile = File(...)):
    raw = await file.read()
    image, _ = _decode_image(raw)
    loop = asyncio.get_running_loop()
    transcoded = await loop.run_in_executor(None, _encode_jpeg, image, 90)
    return Response(content=transcoded, media_type="image/jpeg")


@app.post("/prepare/image")
async def prepare_image(
    file: UploadFile = File(...),
    max_bytes: int = Form(...),
):
    if max_bytes <= 0:
        raise HTTPException(status_code=400, detail="max_bytes must be positive")

    raw = await file.read()
    image, source_mime = _decode_image(raw)
    if len(raw) <= max_bytes:
        media_type = file.content_type or source_mime
        return Response(content=raw, media_type=media_type)

    loop = asyncio.get_running_loop()
    try:
        prepared = await loop.run_in_executor(None, _prepare_image_bytes, image, max_bytes)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc

    return Response(content=prepared, media_type="image/jpeg")


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


class ClassifyEmbeddingsRequest(BaseModel):
    embeddings: list[list[float]]


class ClassifyEmbeddingsResponse(BaseModel):
    results: list[SafetyScores]


@app.post("/classify/safety/embeddings", response_model=ClassifyEmbeddingsResponse)
async def classify_safety_embeddings(req: ClassifyEmbeddingsRequest):
    """Backfill endpoint: score already-computed embeddings without running the
    SigLIP2 backbone again. Pass raw (un-normalized) 768-d vectors as stored in
    visual_identity.embedding — the server normalizes before running heads."""
    if not _any_safety_head_loaded():
        raise HTTPException(status_code=503, detail="no safety heads loaded")
    if not req.embeddings:
        return ClassifyEmbeddingsResponse(results=[])

    arr = np.asarray(req.embeddings, dtype=np.float32)
    if arr.ndim != 2 or arr.shape[1] != EMBEDDING_DIM:
        raise HTTPException(
            status_code=400,
            detail=f"embeddings must be a 2D array with {EMBEDDING_DIM} columns",
        )
    normalized = _l2_normalize(arr)
    loop = asyncio.get_running_loop()
    scores = await loop.run_in_executor(None, _classify_safety, normalized)
    return ClassifyEmbeddingsResponse(results=[SafetyScores(**s) for s in scores])


class ClassifyJunkResponse(BaseModel):
    results: list[float]


@app.post("/classify/junk/embeddings", response_model=ClassifyJunkResponse)
async def classify_junk_embeddings(req: ClassifyEmbeddingsRequest):
    """Backfill endpoint: junk-score already-computed embeddings without running
    the SigLIP2 backbone again. Pass raw (un-normalized) 768-d vectors as stored
    in visual_identity.embedding — the server normalizes before running the head."""
    if _junk_head is None:
        raise HTTPException(status_code=503, detail="no feed junk head loaded")
    if not req.embeddings:
        return ClassifyJunkResponse(results=[])

    arr = np.asarray(req.embeddings, dtype=np.float32)
    if arr.ndim != 2 or arr.shape[1] != EMBEDDING_DIM:
        raise HTTPException(
            status_code=400,
            detail=f"embeddings must be a 2D array with {EMBEDDING_DIM} columns",
        )
    normalized = _l2_normalize(arr)
    loop = asyncio.get_running_loop()
    scores = await loop.run_in_executor(None, _score_junk, normalized)
    return ClassifyJunkResponse(results=scores)


@app.get("/health")
async def health():
    with _umap_lock:
        umap_loaded = _umap_model is not None
    return {
        "status": "ok",
        "device": DEVICE,
        "model": CHECKPOINT,
        "umap": umap_loaded,
        "safety_heads": {axis: (sess is not None) for axis, sess in _safety_heads.items()},
        "junk_head": _junk_head is not None,
        "queues": {
            "text": {"pending": text_queue.qsize(), "max": TEXT_QUEUE_SIZE},
            "image": {"pending": image_queue.qsize(), "max": IMAGE_QUEUE_SIZE},
        },
        "batches": {
            "text": {"min": TEXT_MIN_BATCH, "max": TEXT_MAX_BATCH, "wait_ms": int(TEXT_MAX_WAIT_SECS * 1000)},
            "image": {"max": IMAGE_MAX_BATCH, "wait_ms": int(IMAGE_MAX_WAIT_SECS * 1000)},
        },
    }
