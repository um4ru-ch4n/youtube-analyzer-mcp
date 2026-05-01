"""FastAPI sidecar for CLIP-based frame classification."""

import os
import logging

import torch
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from PIL import Image
from transformers import CLIPProcessor, CLIPModel

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("clip-sidecar")

MODEL_NAME = os.getenv("CLIP_MODEL", "openai/clip-vit-large-patch14")

log.info("Loading CLIP model=%s", MODEL_NAME)
model = CLIPModel.from_pretrained(MODEL_NAME)
processor = CLIPProcessor.from_pretrained(MODEL_NAME)
log.info("Model loaded successfully")

LABELS = ["talking head webcam", "slide with text", "code or terminal", "diagram schema chart", "other"]
LABEL_MAP = {
    "talking head webcam": "talking_head",
    "slide with text": "slide",
    "code or terminal": "code",
    "diagram schema chart": "diagram",
    "other": "other",
}

app = FastAPI(title="CLIP Sidecar")


class ClassifyRequest(BaseModel):
    image_path: str


class ClassifyBatchRequest(BaseModel):
    image_paths: list[str]


class ClassifyResponse(BaseModel):
    type: str
    confidence: float


def classify_image(image_path: str) -> ClassifyResponse:
    if not os.path.exists(image_path):
        raise HTTPException(status_code=400, detail=f"File not found: {image_path}")

    image = Image.open(image_path).convert("RGB")
    inputs = processor(text=LABELS, images=image, return_tensors="pt", padding=True)

    with torch.no_grad():
        outputs = model(**inputs)

    logits = outputs.logits_per_image[0]
    probs = logits.softmax(dim=0)

    best_idx = probs.argmax().item()
    best_label = LABELS[best_idx]
    best_confidence = round(probs[best_idx].item(), 4)

    return ClassifyResponse(
        type=LABEL_MAP[best_label],
        confidence=best_confidence,
    )


@app.post("/classify", response_model=ClassifyResponse)
def classify(req: ClassifyRequest):
    log.info("Classifying %s", req.image_path)
    return classify_image(req.image_path)


@app.post("/classify_batch", response_model=list[ClassifyResponse])
def classify_batch(req: ClassifyBatchRequest):
    log.info("Classifying batch of %d images", len(req.image_paths))
    results = []
    for path in req.image_paths:
        results.append(classify_image(path))
    return results


@app.post("/health")
def health():
    return {"status": "ok"}
