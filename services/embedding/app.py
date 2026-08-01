"""CPU-only ONNX embedding sidecar.

Owns exactly two bounded operations: embed one query, and embed a bounded batch
of passages. Loads the pinned ONNX model once at startup, exposes readiness only
after loading succeeds, and runs on the private Compose network with no public
endpoint. Vectors are mean-pooled and L2-normalized so cosine similarity is a
plain dot product.
"""

import asyncio
import json
import os
from pathlib import Path

import numpy as np
import onnxruntime as ort
import tokenizers
from fastapi import FastAPI, HTTPException, Request, status
from pydantic import BaseModel

MODEL_DIR = Path(os.environ.get("SDDS_EMBEDDING_MODEL_DIR", "/opt/model"))
MODEL_ID = "PORTULAN/serafim-100m-portuguese-pt-sentence-encoder-ir"
MODEL_REVISION = "f27c45d197ea6541dd071b1d992ec91776ee76bd"
DIMENSION = 768
MAX_SEQ_LENGTH = 128

MAX_BATCH = int(os.environ.get("SDDS_EMBEDDING_MAX_BATCH", "32"))
MAX_TEXT_CHARS = int(os.environ.get("SDDS_EMBEDDING_MAX_TEXT_CHARS", "4200"))
MAX_REQUEST_BYTES = int(os.environ.get("SDDS_EMBEDDING_MAX_REQUEST_BYTES", "262144"))
CONCURRENCY = int(os.environ.get("SDDS_EMBEDDING_CONCURRENCY", "1"))
THREADS = int(os.environ.get("SDDS_EMBEDDING_THREADS", "2"))
ACQUIRE_TIMEOUT = 10.0

_lock_path = MODEL_DIR / "model.lock.json"
if _lock_path.exists():
    _lock = json.loads(_lock_path.read_text(encoding="utf-8"))
    _MODEL_ID = _lock.get("model_id", MODEL_ID)
    _MODEL_REVISION = _lock.get("model_revision", MODEL_REVISION)
    _DIMENSION = int(_lock.get("dimension", DIMENSION))
else:  # pragma: no cover - the Docker image always ships model.lock.json
    _MODEL_ID = MODEL_ID
    _MODEL_REVISION = MODEL_REVISION
    _DIMENSION = DIMENSION

_session_options = ort.SessionOptions()
_session_options.intra_op_num_threads = THREADS
_session_options.inter_op_num_threads = 1
_session = ort.InferenceSession(
    str(MODEL_DIR / "model.onnx"),
    providers=["CPUExecutionProvider"],
    sess_options=_session_options,
)

_tokenizer_path = MODEL_DIR / "tokenizer.json"
if not _tokenizer_path.exists():  # pragma: no cover - image always ships it
    raise RuntimeError("tokenizer.json not found beside the ONNX model")
_tokenizer = tokenizers.Tokenizer.from_file(str(_tokenizer_path))
_tokenizer.enable_truncation(max_length=MAX_SEQ_LENGTH)
# Pad to each batch's longest sequence; the length is resolved at encode time.
_tokenizer.enable_padding(length=None, pad_id=0, pad_token="[PAD]")

_semaphore = asyncio.Semaphore(CONCURRENCY)


class EmbeddingRequest(BaseModel):
    texts: list[str]


class EmbeddingResponse(BaseModel):
    model_id: str
    model_revision: str
    dimension: int
    vectors: list[list[float]]


app = FastAPI(title="sdds-embedding")


@app.get("/healthz")
async def healthz():
    return {
        "status": "ok",
        "model_id": _MODEL_ID,
        "model_revision": _MODEL_REVISION,
        "dimension": _DIMENSION,
    }


def _embed(texts: list[str]) -> np.ndarray:
    encoded = _tokenizer.encode_batch(texts)
    input_ids = np.array([e.ids for e in encoded], dtype=np.int64)
    attention_mask = np.array([e.attention_mask for e in encoded], dtype=np.int64)
    # The ONNX export bundles the sentence-transformers Pooling module, so
    # output[1] (sentence_embedding) is already mean-pooled; we only normalize.
    outputs = _session.run(
        None,
        {"input_ids": input_ids, "attention_mask": attention_mask},
    )
    sentence_embedding = outputs[1]
    norms = np.linalg.norm(sentence_embedding, axis=1, keepdims=True)
    norms[norms == 0] = 1.0
    return sentence_embedding / norms

@app.post("/v1/embeddings", response_model=EmbeddingResponse)
async def embed(request: Request, body: EmbeddingRequest):
    raw = await request.body()
    if len(raw) > MAX_REQUEST_BYTES:
        raise HTTPException(status_code=status.HTTP_413_REQUEST_ENTITY_TOO_LARGE, detail={"error": "too_large"})
    if not body.texts:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail={"error": "invalid_request"})
    if len(body.texts) > MAX_BATCH:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail={"error": "invalid_request"})
    for text in body.texts:
        if len(text) > MAX_TEXT_CHARS:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail={"error": "invalid_request"})

    try:
        await asyncio.wait_for(_semaphore.acquire(), timeout=ACQUIRE_TIMEOUT)
    except asyncio.TimeoutError:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail={"error": "unavailable"})
    try:
        vectors = await asyncio.to_thread(_embed, body.texts)
    finally:
        _semaphore.release()

    return EmbeddingResponse(
        model_id=_MODEL_ID,
        model_revision=_MODEL_REVISION,
        dimension=_DIMENSION,
        vectors=vectors.tolist(),
    )
