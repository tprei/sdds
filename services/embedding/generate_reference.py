"""Generate the PT-BR reference embeddings fixture.

Run once, locally, by hand -- never in CI::

    python services/embedding/generate_reference.py

It loads the pinned model with sentence-transformers (PyTorch), encodes the
fixture sentences with the documented query/passage behavior (no prompts, mean
pooling, then L2 normalization), and writes testdata/reference_embeddings.json.
The committed vectors are the ground truth the ONNX export must reproduce within
the parity tolerance asserted by test_reference_parity.py.
"""

import datetime
import json
import platform
from pathlib import Path

import sentence_transformers
import torch
from sentence_transformers import SentenceTransformer

from export_model import MODEL_ID, MODEL_REVISION

FIXTURE_SENTENCES = [
    # A vocabulary-mismatch pair: query and note share no lexical tokens.
    "lugar bom pra trabalhar de notebook",
    "Wi-Fi estável, várias tomadas e ninguém reclamou que fiquei duas horas",
    # Accents and Brazilian informal phrasing.
    "Café da esquina tem pão de queijo quentinho toda manhã",
    "Achei um achadinho baratinho no camelô da feira",
    # An exact business / product name that lexical search must still find.
    "Comprei na Padaria Pão Quente o melhor bauru da cidade",
    # Short query-shaped strings.
    "onde tem wifi",
    "lugar pra estudar",
    "tomada livre",
]

REFERENCE_PATH = Path(__file__).resolve().parent / "testdata" / "reference_embeddings.json"
PROVENANCE_PATH = Path(__file__).resolve().parent / "testdata" / "reference_embeddings.provenance.json"


def l2_normalize(matrix):
    norms = (matrix * matrix).sum(axis=1, keepdims=True) ** 0.5
    return matrix / norms


def main():
    model = SentenceTransformer(MODEL_ID, revision=MODEL_REVISION)
    # The model's documented max_seq_length is 128; make the encoder honor it so
    # the reference matches what the ONNX export truncates to.
    model.max_seq_length = 128
    raw = model.encode(
        FIXTURE_SENTENCES,
        normalize_embeddings=False,
        convert_to_numpy=True,
    )
    vectors = l2_normalize(raw).tolist()

    REFERENCE_PATH.write_text(
        json.dumps({"texts": FIXTURE_SENTENCES, "vectors": vectors}, ensure_ascii=False, indent=2)
        + "\n",
        encoding="utf-8",
    )

    provenance = {
        "model_id": MODEL_ID,
        "model_revision": MODEL_REVISION,
        "sentence_transformers_version": sentence_transformers.__version__,
        "torch_version": torch.__version__,
        "generated_at": datetime.date.today().isoformat(),
        "python_version": platform.python_version(),
    }
    PROVENANCE_PATH.write_text(
        json.dumps(provenance, indent=2) + "\n", encoding="utf-8"
    )

    print(f"wrote {REFERENCE_PATH}")
    print(f"wrote {PROVENANCE_PATH}")


if __name__ == "__main__":
    main()
