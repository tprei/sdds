"""Export the pinned Portuguese sentence-embedding model to ONNX.

Run once during the Docker image build (the ``export`` stage). The model id and
revision are module constants that must match the Go constants in
services/api/internal/note/embedding.go exactly. Production startup never
downloads an unpinned revision: the weights are baked into the image here.
"""

from optimum.exporters.onnx import main_export

MODEL_ID = "PORTULAN/serafim-100m-portuguese-pt-sentence-encoder-ir"
MODEL_REVISION = "f27c45d197ea6541dd071b1d992ec91776ee76bd"
MAX_SEQ_LENGTH = 128

if __name__ == "__main__":
    # feature-extraction is the sentence-transformers/transformers task that
    # yields last_hidden_state; mean-pooling happens in app.py. Opset 17 is
    # supported by the pinned onnxruntime runtime version.
    main_export(
        model_name_or_path=MODEL_ID,
        revision=MODEL_REVISION,
        task="feature-extraction",
        opset=17,
        output="/export",
    )
