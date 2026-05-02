# youtube-analyzer-mcp

MCP server for YouTube video analysis — transcripts + visual content (slides, code, diagrams).

## Documentation

- `docs/SPEC.md` — full technical specification
- `docs/ARCHITECTURE.md` — architecture plan
- `README.md` — setup guide and usage

## Stack

Go 1.25, mcp-go (mark3labs), zap (uber), viper (spf13), modernc.org/sqlite, Ollama (Qwen3 8B)

## Architecture

```
cmd/server/main.go              — entry point, transport (stdio/HTTP)
internal/
├── app/app.go                   — MCP server creation, tool registration, lifecycle
├── config/config.go             — viper config.yaml + env vars
├── handler/mcp/                 — transport layer (MCP request → DTO → service → response)
│   ├── deps.go                  — TaskService interface
│   ├── handler.go               — Handler + RegisterTools (6 tools)
│   ├── add_video.go             — submit video URL
│   ├── get_status.go            — poll task progress
│   ├── get_result.go            — text summaries + artifact metadata (no images)
│   ├── get_artifacts.go         — images for a specific chunk by index
│   ├── delete_task.go           — remove task + artifacts
│   ├── retry_task.go            — re-enqueue failed task from checkpoint
│   └── helpers.go
├── service/task/                — TaskManager: queue + worker pool
│   ├── deps.go                  — TaskRepository, PipelineRunner interfaces
│   ├── manager.go               — Submit, GetStatus, GetResult, Delete, Retry
│   └── worker.go
├── service/chunking/            — time-based chunking (30-sec chunks)
├── usecase/pipeline/            — pipeline orchestrator + steps
│   ├── deps.go                  — all adapter interfaces
│   ├── runner.go                — orchestrator (parallel fan-out, checkpoint/resume)
│   ├── state.go                 — PipelineState + checkpoint persistence
│   ├── step_download.go         — yt-dlp (video + audio + subtitles)
│   ├── step_transcribe.go       — subtitles first, Whisper fallback
│   ├── step_extract_frames.go   — FFmpeg + pHash dedup, dynamic min frames
│   ├── step_process_frames.go   — CLIP classify → OCR slide/code, Vision diagram/other
│   ├── step_chunk.go            — merge transcript into time chunks
│   └── step_summarize.go        — Qwen3 8B (speaker voice, 5x compression) + match artifacts
├── adapter/                     — external tool implementations
│   ├── ytdlp/                   — yt-dlp subprocess (download + subtitles)
│   ├── subtitle/                — VTT parser (YouTube auto-subs)
│   ├── whisper/                 — HTTP client to whisper sidecar
│   ├── ffmpeg/                  — frame extraction (scene detect + interval fallback, ffprobe duration)
│   ├── clip/                    — HTTP client to CLIP sidecar
│   ├── ocr/                     — tesseract subprocess
│   ├── phash/                   — perceptual hash dedup
│   ├── vision/                  — Gemini/Claude API (IsUsefulFrame doklassification)
│   └── summarizer/              — Ollama HTTP client (speaker-voice compression)
├── model/                       — domain models
│   ├── task.go                  — Task, TaskStatus, TaskResult, Warning
│   ├── video.go                 — VideoMeta, DownloadResult (with SubtitlesPath)
│   ├── frame.go                 — Frame, ProcessedFrame (Useful, OCRText, ImagePath)
│   ├── chunk.go                 — Chunk, ChunkSummary, Artifact
│   └── errors.go
├── repository/task/             — SQLite (busy_timeout, WAL mode, SetMaxOpenConns(1))
└── container/container.go       — DI wiring
sidecar/
├── whisper/                     — FastAPI + faster-whisper (large-v3-turbo)
└── clip/                        — FastAPI + CLIP (ViT-L/14)
pkg/logger/                      — zap structured logger with context
```

## Pipeline v2 (current)

Two parallel branches:
1. **Text branch:** Subtitles/Whisper → 30-sec chunks → Qwen3 8B summarization (speaker's voice, 5x compression, video title context)
2. **Visual branch:** FFmpeg frames → pHash dedup → CLIP classify → discard talking_head → OCR for slide/code → Vision API doklassification for diagram/other → keep useful frames as Artifacts

Results merged by timestamp. Artifacts returned via separate `get_artifacts` tool to save tokens.

## Commands

```bash
go build ./cmd/server                    # build
go test ./...                            # unit tests
docker-compose up -d mcp-server whisper clip  # start all services
docker-compose logs -f mcp-server        # follow logs
```

## Key Principles

- No else/else-if — only early return and separate if blocks
- Return values, don't mutate via pointer arguments
- Interfaces at point of use (deps.go in consumer package)
- MCP server init in app.go, handler = pure transport layer
- Tests next to source files (_test.go)
- Two separate AI providers: VisionAnalyzer (external API) + Summarizer (local Ollama)

## Running

MCP server runs in Docker (HTTP mode, port 39280). Ollama runs natively on host for GPU.

Claude Code config (`~/.claude.json`):
```json
"youtube-analyzer": {
  "type": "http",
  "url": "http://localhost:39280/mcp"
}
```

## Data Structure

```
data/tasks/{task_id}/
├── video.mp4, audio.wav, subs.en.vtt
├── frames/                    — extracted frames (JPG)
├── processing/                — intermediate results (JSON)
│   ├── transcript.json        — subtitle/whisper segments
│   ├── frames_deduped.json    — after pHash
│   ├── frames_classified.json — after CLIP + OCR
│   ├── chunks.json            — transcript chunks
│   ├── summaries.json         — summaries + artifacts
│   └── result.json            — final result
└── checkpoint.json            — pipeline resume point
```

## Current Limitations

- Vision API (Gemini/Claude) not configured — diagram frames pass through unfiltered
- Whiteboard videos produce many similar frames (temporal dedup not yet implemented)
- FFmpeg scene detection often falls back to interval mode in Docker (exit 254)
- Frame extraction at 5-sec intervals — needs GPU for higher frequency
- No automatic task cleanup (manual via delete_task)
