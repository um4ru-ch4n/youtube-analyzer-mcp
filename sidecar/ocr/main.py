"""FastAPI sidecar for GPU-accelerated OCR using EasyOCR."""

import os
import logging

import torch
import easyocr
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("ocr-sidecar")

LANGUAGES = os.getenv("OCR_LANGUAGES", "en,ru").split(",")
USE_GPU = torch.cuda.is_available()

log.info("Loading EasyOCR languages=%s gpu=%s", LANGUAGES, USE_GPU)
reader = easyocr.Reader(LANGUAGES, gpu=USE_GPU)
log.info("EasyOCR loaded successfully (gpu=%s)", USE_GPU)

app = FastAPI(title="OCR Sidecar")


class OCRRequest(BaseModel):
    image_path: str


class OCRResponse(BaseModel):
    text: str


def _release_vram():
    if USE_GPU:
        torch.cuda.empty_cache()


@app.post("/read", response_model=OCRResponse)
def read(req: OCRRequest):
    if not os.path.exists(req.image_path):
        raise HTTPException(status_code=400, detail=f"File not found: {req.image_path}")

    log.info("OCR %s", req.image_path)
    try:
        results = reader.readtext(req.image_path, detail=0, paragraph=True)
        text = "\n".join(results)
        return OCRResponse(text=text)
    finally:
        _release_vram()


@app.post("/release_vram")
def release_vram():
    """Manually trigger CUDA cache release."""
    _release_vram()
    return {"status": "ok"}


@app.post("/health")
def health():
    return {"status": "ok"}
