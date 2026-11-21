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
    language: str | None = None
    initial_prompt: str | None = None
    hotwords: str | None = None
    # Defaults keep the pre-VAD behaviour for clients that do not send these fields.
    vad_filter: bool = False
    vad_min_silence_ms: int = 2000
    condition_on_previous_text: bool = True


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

    log.info(
        "Transcribing %s language=%s initial_prompt=%d chars hotwords=%d chars "
        "vad=%s min_silence=%dms condition_on_previous_text=%s",
        req.audio_path,
        req.language or "auto",
        len(req.initial_prompt or ""),
        len(req.hotwords or ""),
        req.vad_filter,
        req.vad_min_silence_ms,
        req.condition_on_previous_text,
    )

    segments_iter, info = model.transcribe(
        req.audio_path,
        beam_size=5,
        language=req.language or None,
        initial_prompt=req.initial_prompt or None,
        hotwords=req.hotwords or None,
        vad_filter=req.vad_filter,
        vad_parameters={"min_silence_duration_ms": req.vad_min_silence_ms, "speech_pad_ms": 400}
        if req.vad_filter
        else None,
        condition_on_previous_text=req.condition_on_previous_text,
    )

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
