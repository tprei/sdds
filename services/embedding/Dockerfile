# syntax=docker/dockerfile:1.7

# ----- export: produce the pinned ONNX model + tokenizer (build-only) -----
FROM python:3.12-slim AS export
ENV PIP_NO_CACHE_DIR=1 PIP_DISABLE_PIP_VERSION_CHECK=1
WORKDIR /work

# Install export dependencies before copying source so the layer is reused
# across edits to export_model.py / app.py.
COPY requirements-export.txt .
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install -r requirements-export.txt

COPY export_model.py model.lock.json ./
RUN python export_model.py

# optimum does not always copy the fast tokenizer, so fetch it from the same
# pinned revision the model was exported from.
RUN python - <<'PY'
import json, urllib.request
lock = json.load(open("/work/model.lock.json"))
revision = lock["model_revision"]
url = f"https://huggingface.co/{lock['model_id']}/resolve/{revision}/tokenizer.json"
urllib.request.urlretrieve(url, "/export/tokenizer.json")
PY

# Gate: the exported ONNX weights must match the pinned digest. If the export
# is not byte-reproducible, set onnx_sha256 to null in model.lock.json (with a
# one-line reason) and rely on test_reference_parity.py instead.
RUN python - <<'PY'
import hashlib, json, sys
lock = json.load(open("/work/model.lock.json"))
expected = lock.get("onnx_sha256")
if not expected:
    print("onnx_sha256 not pinned; skipping digest gate")
    sys.exit(0)
actual = hashlib.sha256(open("/export/model.onnx", "rb").read()).hexdigest()
if actual != expected:
    sys.exit(f"onnx sha256 mismatch: export={actual} lock={expected}")
print(f"onnx sha256 verified: {actual}")
PY

# ----- runtime-base: the shared image the final and test stages build on -----
FROM python:3.12-slim AS runtime-base
ENV PIP_NO_CACHE_DIR=1 PIP_DISABLE_PIP_VERSION_CHECK=1 PYTHONUNBUFFERED=1
WORKDIR /app

COPY requirements.txt .
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install -r requirements.txt

COPY --from=export /export /opt/model
COPY model.lock.json /opt/model/model.lock.json
COPY app.py ./


# ----- test: parity check against the committed reference embeddings -----
FROM runtime-base AS test
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install pytest
COPY test_reference_parity.py ./
COPY testdata ./testdata
RUN python -m pytest -q
# ----- final: the production sidecar image -----
FROM runtime-base
USER 10001:10001
EXPOSE 8081
ENV SDDS_EMBEDDING_ADDR=:8081 SDDS_EMBEDDING_MODEL_DIR=/opt/model
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8081"]
