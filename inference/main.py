# main.py
import asyncio
import io
from contextlib import asynccontextmanager
from concurrent.futures import ThreadPoolExecutor

import torch
from fastapi import FastAPI, File, UploadFile, HTTPException
from PIL import Image
from pydantic import BaseModel
from transformers import AutoModel, AutoProcessor

# ── Config ────────────────────────────────────────────────────────────────────

CHECKPOINT    = "google/siglip2-base-patch16-naflex"
DEVICE        = "mps"
DTYPE         = torch.bfloat16
MAX_PATCHES   = 256
MIN_BATCH     = 8
MAX_BATCH     = 32
MAX_WAIT_SECS = 0.020

# ── State ─────────────────────────────────────────────────────────────────────

model     = None
processor = None
executor  = ThreadPoolExecutor(max_workers=1)

text_queue: asyncio.Queue = asyncio.Queue()

# ── Inference ─────────────────────────────────────────────────────────────────

def _embed_image(image: Image.Image) -> list[float]:
    inputs = processor(
        images=[image],
        max_num_patches=MAX_PATCHES,
        return_tensors="pt",
    ).to(DEVICE)
    with torch.no_grad():
        features = model.get_image_features(**inputs)
    return features.cpu().float().numpy().tolist()[0]


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
    return features.cpu().float().numpy().tolist()

# ── Text batch worker ─────────────────────────────────────────────────────────

async def _text_batch_worker():
    loop = asyncio.get_event_loop()
    while True:
        first_text, first_future = await text_queue.get()
        batch   = [first_text]
        futures = [first_future]

        deadline = loop.time() + MAX_WAIT_SECS
        try:
            while len(batch) < MAX_BATCH:
                timeout = deadline - loop.time()
                if timeout <= 0:
                    break
                text, fut = await asyncio.wait_for(text_queue.get(), timeout=timeout)
                batch.append(text)
                futures.append(fut)
                if len(batch) >= MIN_BATCH:
                    break
        except asyncio.TimeoutError:
            pass

        try:
            results = await loop.run_in_executor(executor, _embed_texts, batch)
            for fut, result in zip(futures, results):
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

    asyncio.create_task(_text_batch_worker())

    yield

    executor.shutdown(wait=False)

# ── App ───────────────────────────────────────────────────────────────────────

app = FastAPI(lifespan=lifespan)

# ── Routes ────────────────────────────────────────────────────────────────────

class TextRequest(BaseModel):
    text: str

class EmbeddingResponse(BaseModel):
    embedding: list[float]

@app.post("/embed/image", response_model=EmbeddingResponse)
async def embed_image(file: UploadFile = File(...)):
    if not file.content_type.startswith("image/"):
        raise HTTPException(status_code=400, detail="File must be an image")

    raw = await file.read()
    try:
        image = Image.open(io.BytesIO(raw)).convert("RGB")
    except Exception:
        raise HTTPException(status_code=400, detail="Could not decode image")

    loop = asyncio.get_event_loop()
    embedding = await loop.run_in_executor(executor, _embed_image, image)
    return EmbeddingResponse(embedding=embedding)


@app.post("/embed/text", response_model=EmbeddingResponse)
async def embed_text(req: TextRequest):
    loop   = asyncio.get_event_loop()
    future = loop.create_future()
    await text_queue.put((req.text, future))
    embedding = await future
    return EmbeddingResponse(embedding=embedding)


@app.get("/health")
async def health():
    return {"status": "ok", "device": DEVICE, "model": CHECKPOINT}