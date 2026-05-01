"""FastAPI sidecar for faster-whisper transcription."""

import os
import logging

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from faster_whisper import WhisperModel

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("whisper-sidecar")

MODEL_SIZE = os.getenv("WHISPER_MODEL", "large-v3-turbo")
DEVICE = os.getenv("WHISPER_DEVICE", "cpu")
COMPUTE_TYPE = os.getenv("WHISPER_COMPUTE_TYPE", "int8")

log.info(
    "Loading whisper model=%s device=%s compute_type=%s",
    MODEL_SIZE,
    DEVICE,
    COMPUTE_TYPE,
)
model = WhisperModel(MODEL_SIZE, device=DEVICE, compute_type=COMPUTE_TYPE)
log.info("Model loaded successfully")

app = FastAPI(title="Whisper Sidecar")


class TranscribeRequest(BaseModel):
    audio_path: str


class Segment(BaseModel):
    start: float
    end: float
    text: str


class TranscribeResponse(BaseModel):
    segments: list[Segment]
    language: str


@app.post("/transcribe", response_model=TranscribeResponse)
def transcribe(req: TranscribeRequest):
    if not os.path.exists(req.audio_path):
        raise HTTPException(status_code=400, detail=f"File not found: {req.audio_path}")

    log.info("Transcribing %s", req.audio_path)

    segments_iter, info = model.transcribe(req.audio_path, beam_size=5)

    segments = []
    for seg in segments_iter:
        segments.append(
            Segment(start=round(seg.start, 3), end=round(seg.end, 3), text=seg.text.strip())
        )

    log.info(
        "Transcription complete: %d segments, language=%s (prob=%.2f)",
        len(segments),
        info.language,
        info.language_probability,
    )

    return TranscribeResponse(segments=segments, language=info.language)


@app.post("/health")
def health():
    return {"status": "ok"}
