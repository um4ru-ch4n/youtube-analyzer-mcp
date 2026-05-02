# YouTube Video Analysis MCP Server — Technical Specification (v2)

## 1. Overview

### Goal

Create an MCP server that allows AI agents (Claude, ChatGPT, or any MCP-compatible model) to analyze YouTube videos and extract structured information — not just speech transcripts, but also visual content: slides, code on screen, diagrams, schemas.

### Problem Statement

Existing YouTube analysis tools (NoteGPT, Glasp, Eightify) work only with subtitles. For technical content — tutorials, lectures, demos — significant information is conveyed visually: code in editors, diagrams, slide transitions. Without visual information, a subtitle-based summary is incomplete and often incomprehensible.

This project fills this gap: the server analyzes both the audio track (transcript with timestamps) and the video track (key frames), merges them into a unified structure, and returns the full video context to the AI agent.

### How It Works (High Level)

1. User invokes MCP tool with YouTube URL via AI agent
2. Server launches async pipeline: transcription and frame extraction run in parallel
3. Transcript is split into time-based chunks (~45 seconds each) with overlap for context preservation
4. Key frames are processed based on type: OCR for slides and code, Vision analysis for diagrams
5. Each chunk gets associated frames (by timestamps), then a summary is generated
6. Final result: array of chunk objects with time range, summary, and textual frame content
7. The calling AI agent receives the result and decides what to do: build notes, answer questions, generate quizzes, etc.

### Technical Stack

- **Language:** Go
- **Protocol:** MCP (Model Context Protocol), library: `github.com/mark3labs/mcp-go`
- **Transcription:** faster-whisper (`large-v3-turbo`), runs as **sidecar container** (FastAPI)
- **Video download:** yt-dlp (subprocess)
- **Frame extraction:** FFmpeg (subprocess, scene detection + interval fallback)
- **Frame deduplication:** perceptual hash (pHash) — applied BEFORE classification
- **Frame classification:** CLIP, runs as **sidecar container** (FastAPI)
- **OCR:** Tesseract (subprocess)
- **Vision analysis (diagrams):** external AI API (Gemini or Claude) via adapter pattern
- **Summarization:** local LLM via **Ollama sidecar** (Qwen3 8B) — separate from Vision provider
- **Storage:** SQLite (`modernc.org/sqlite`, pure Go) + filesystem for artifacts
- **Deploy:** Docker + Docker Compose, configuration via ENV + config.yaml

### Key Principles

- Server runs **locally**, no public hosting planned (v1)
- All heavy ML tools (Whisper, CLIP, Ollama) run **locally** as sidecar containers — minimal external API costs
- Vision analysis (diagrams) is the only step requiring external API, applied only to `diagram` and `other` frame types
- Server is **asynchronous**: `add_video` returns `task_id` immediately, processing runs in background
- Support for multiple concurrent tasks (worker pool + queue)
- **Stateful**: results persist on disk (SQLite + filesystem)
- **Two separate AI providers**: VisionAnalyzer (image analysis, external API) and Summarizer (text summaries, local LLM)

---

## 2. Architecture

```
+-------------------------------------------------------------+
|                     MCP Server (Go)                          |
|                                                              |
|  main.go / app.go                                            |
|  +------------+  +-----------+  +----------+  +----------+  |
|  | add_video  |  | get_status|  | get_result|  |delete_task| |
|  +-----+------+  +-----+-----+  +-----+----+  +----+-----+ |
|        |               |              |              |       |
|        +---------- Handler Layer (transport) --------+       |
|                         |                                    |
|              Task Manager (queue + worker pool)              |
|                         |                                    |
|  +----------------------v------------------------------+     |
|  |                   Pipeline                          |     |
|  |                                                     |     |
|  |  DownloadStep (yt-dlp)                              |     |
|  |       |                        |                    |     |
|  |  TranscribeStep           FrameExtractStep          |     |
|  |  (whisper sidecar)        (ffmpeg + phash + CLIP)   |     |
|  |       |                        |                    |     |
|  |       |                  FrameProcessStep           |     |
|  |       |                  (OCR / Vision API)         |     |
|  |       |                        |                    |     |
|  |       +-------- ChunkStep -----+                    |     |
|  |                     |                               |     |
|  |              SummarizeStep (Ollama)                  |     |
|  |                     |                               |     |
|  |               ResultStep                            |     |
|  +-----------------------------------------------------+    |
|                                                              |
|  Adapter Interfaces                                          |
|  +------------------+  +------------------+                  |
|  | Vision: Gemini   |  | Vision: Claude   |                  |
|  +------------------+  +------------------+                  |
|  +------------------+                                        |
|  | Summarizer: Ollama|                                       |
|  +------------------+                                        |
+--------------------------------------------------------------+
```

**Key principles:**
- Transcription and frame extraction execute **in parallel** (errgroup)
- MCP server initialized in `main.go`/`app.go`, NOT in handler
- Handler is pure transport layer: MCP request → DTO → service → response
- Interfaces defined at point of use (`deps.go` in consumer package)
- Vision provider (image analysis) and Summarizer (text summaries) are **separate** — configured independently via ENV

---

## 3. MCP Tools (Public Interface)

### `add_video`
Add video to processing queue.

**Input:**
```json
{ "url": "https://youtube.com/watch?v=..." }
```

**Output:**
```json
{ "task_id": "550e8400-e29b-41d4-a716-446655440000" }
```

### `get_status`
Get current task status.

**Input:**
```json
{ "task_id": "550e8400-e29b-41d4-a716-446655440000" }
```

**Output:**
```json
{
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued | downloading | transcribing | extracting_frames | processing_frames | chunking | summarizing | completed | failed",
  "progress": "extracting_frames",
  "error": "error description if status=failed",
  "warnings": ["OCR failed on frame at 45.2s: tesseract timeout"]
}
```

### `get_result`
Get analysis result for completed task.

**Input:**
```json
{ "task_id": "550e8400-e29b-41d4-a716-446655440000" }
```

**Output:**
```json
{
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "video_url": "https://youtube.com/watch?v=...",
  "video_title": "Video Title",
  "duration_seconds": 1823,
  "chunks": [
    {
      "index": 0,
      "time_start": 0.0,
      "time_end": 45.0,
      "summary": "Author explains merge sort basics. Complexity O(n log n). Diagram shows array splitting.",
      "frames": [
        {
          "timestamp_sec": 12.5,
          "frame_type": "diagram",
          "content": "Array splitting: [8,3,1,5] -> [8,3] + [1,5] -> [8]+[3]+[1]+[5]"
        },
        {
          "timestamp_sec": 38.0,
          "frame_type": "code",
          "content": "func mergeSort(arr []int) []int {\n  if len(arr) <= 1 { return arr }\n  ..."
        }
      ]
    }
  ]
}
```

### `delete_task`
Delete task and all its artifacts. Only for `completed` or `failed` tasks.

**Input:**
```json
{ "task_id": "550e8400-e29b-41d4-a716-446655440000" }
```

**Output:**
```json
{ "success": true }
```

> **Note:** Frame images are NOT included in the final result — only textual content (OCR / description / Vision analysis). The calling AI agent gets clean JSON.

---

## 4. Pipeline Steps

### Step 1 — Download
- Tool: `yt-dlp` (subprocess)
- Download audio (for Whisper) and video (for frames) by YouTube URL
- Save to task directory `data/tasks/{task_id}/`
- Extract metadata: title, duration
- **Validation:** check duration <= MAX_VIDEO_DURATION (default: 2 hours)

### Step 2a — Transcription (parallel with 2b)
- Tool: faster-whisper sidecar (HTTP API)
- Model: `large-v3-turbo`
- Get transcript with segment-level timestamps
- Segment format: `{ start_sec, end_sec, text }`

### Step 2b — Frame Extraction & Processing (parallel with 2a)
- **Scene detect:** FFmpeg `select='gt(scene,THRESHOLD)'` — configurable threshold (default 0.3)
- **Fallback:** if scene detection yields too few (<5) or too many (>500) frames, fall back to regular interval (every N seconds)
- **Dedup (before CLIP):** perceptual hash (pHash) — remove visually identical consecutive frames. Threshold configurable (default 0.95)
- **Classification:** CLIP sidecar (HTTP API) — classify each frame:
  - `talking_head` — talking head / webcam
  - `slide` — slide with text
  - `code` — code or terminal
  - `diagram` — diagram, schema, chart
  - `other` — everything else
- Result: list of frames with `{ timestamp_sec, filepath, frame_type }`

### Step 3 — Frame Content Extraction
Processing depends on `frame_type`:

| Type | Method | Tool |
|------|--------|------|
| `talking_head` | Fixed text: "Talking head, no visual information" | None |
| `slide` | OCR | Tesseract (subprocess) |
| `code` | OCR | Tesseract (subprocess) |
| `diagram` | Vision analysis | VisionAnalyzer (Gemini/Claude API) |
| `other` | Vision analysis | VisionAnalyzer (Gemini/Claude API) |

Each frame result: `{ timestamp_sec, frame_type, content: string }`

**Error handling:** If OCR/Vision fails on a frame → log warning, add to `task.warnings[]`, continue pipeline.

### Step 4 — Chunking
- Split transcript into chunks by **target duration** (~45 seconds, configurable)
- Each chunk has `time_start` and `time_end`
- **Overlap:** last ~10 seconds of previous chunk included as context in summarization prompt (NOT in final output)

### Step 5 — Frame-to-Chunk Matching
- For each processed frame, assign to chunk by `timestamp_sec`
- Condition: `chunk.time_start <= frame.timestamp_sec <= chunk.time_end`

### Step 6 — Summarization
- Tool: **Ollama sidecar** with Qwen3 8B (local, free)
- For each chunk: transcript text + frame content → summarization prompt
- Overlap context included in prompt but excluded from output
- Get brief summary: main idea + key facts for the period

### Step 7 — Result Building
- Assemble final result from all chunk summaries
- Save to SQLite (task record) + `data/tasks/{task_id}/result.json`
- Update task status to `completed`
- Clean up temporary files (video, audio — keep frames and result)

---

## 5. Vision & Summarizer Architecture

Two separate interfaces with independent provider selection:

### VisionAnalyzer (image analysis)
- Used only for `diagram` and `other` frame types
- Providers: Gemini API (default, free tier), Claude API (alternative)
- Selected via `VISION_PROVIDER=gemini|claude`
- Minimal API calls per video (only diagram frames after all filtering)

### Summarizer (text chunk summaries)
- Used for every chunk
- Provider: Ollama with Qwen3 8B (default, local, free)
- Selected via `SUMMARY_PROVIDER=ollama` (future: custom server)
- Interface ready for replacement with any HTTP-compatible LLM API

Both are configured independently — changing one doesn't affect the other.

---

## 6. Error Handling & Logging

### Structured logging (zap)
Every error logged with full context:
```
logger.ErrorKV(ctx, "OCR failed on frame",
    "task_id", taskID,
    "step", "process_frames",
    "frame_index", i,
    "frame_path", frame.Path,
    "frame_timestamp", frame.TimestampSec,
    "error", err.Error(),
)
```

### Non-fatal errors
- Log as warning
- Add to `task.warnings[]`
- Continue pipeline
- Examples: OCR timeout on one frame, CLIP low-confidence classification

### Fatal errors
- Log as error with full context
- Set `task.status = "failed"` with descriptive `task.error`
- Clean up temporary files
- AI agent sees error via `get_status` and decides next action
- Examples: download failed (private video), whisper sidecar unreachable, disk full

---

## 7. Storage

### SQLite (task tracking)
- Pure Go driver: `modernc.org/sqlite`
- Tables: `tasks` (id, url, status, progress, error, warnings_json, result_json, created_at, updated_at)
- Atomic updates, simple queries

### Filesystem (artifacts)
```
data/tasks/{task_id}/
├── audio.wav
├── video.mp4
├── frames/
│   ├── frame_001_45.2s.jpg
│   └── ...
└── result.json
```

---

## 8. Configuration

### config.yaml (non-secret)
```yaml
pipeline:
  chunk_duration_sec: 45
  overlap_sec: 10
  max_video_duration_sec: 7200
  max_concurrent_frames: 8
  scene_detect_threshold: 0.3
  scene_detect_min_frames: 5
  scene_detect_max_frames: 500
  fallback_interval_sec: 5
  dedup_similarity_threshold: 0.95

worker_pool:
  size: 2
  queue_size: 100

storage:
  data_dir: "./data"

whisper:
  url: "http://whisper:8001"
  timeout_sec: 300
  model: "large-v3-turbo"

clip:
  url: "http://clip:8002"
  timeout_sec: 30

vision:
  provider: "gemini"
  gemini:
    model: "gemini-2.0-flash"
    timeout_sec: 60
  claude:
    model: "claude-sonnet-4-20250514"
    timeout_sec: 60

summarizer:
  provider: "ollama"
  ollama:
    url: "http://ollama:11434"
    model: "qwen3:8b"
    timeout_sec: 120

ocr:
  languages: ["eng", "rus"]

http:
  port: 3000
  path: "/mcp"
```

### Environment Variables
```
TRANSPORT_MODE=stdio|http
HTTP_PORT=3000
LOG_LEVEL=debug|info|warn|error
APP_ENV=development|production
GEMINI_API_KEY=...
ANTHROPIC_API_KEY=...
```

---

## 9. Docker Compose

```yaml
services:
  mcp-server:
    build: .
    volumes: [shared-data:/data]
    depends_on: [whisper, clip, ollama]

  whisper:
    build: ./sidecar/whisper
    volumes: [shared-data:/data]

  clip:
    build: ./sidecar/clip
    volumes: [shared-data:/data]

  ollama:
    image: ollama/ollama
    volumes: [ollama-models:/root/.ollama, shared-data:/data]

volumes:
  shared-data:
  ollama-models:
```

---

## 10. Limitations (v1)

- Only public YouTube URLs (no playlists)
- Results stored on disk (SQLite + filesystem)
- No automatic cleanup (manual via `delete_task`)
- Frame images not included in final response — only textual content
- Local deployment only (not production-ready)
- MAX_VIDEO_DURATION = 2 hours

---

## 11. Future Improvements (Backlog)

- Support local video files
- Support playlists
- TTL-based automatic task cleanup
- Base64 frame images in result (for multimodal AI agents)
- Web UI for browsing results
- PostgreSQL instead of SQLite
- Custom summarization server (replace Ollama)
- Public hosting
- Research and compile list of domains/subdomains needed for split-tunnel VPN config (googlevideo.com, youtube.com, etc.) so yt-dlp works without routing all traffic through VPN
