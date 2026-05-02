# youtube-analyzer-mcp

MCP server that analyzes YouTube videos — both audio transcripts and visual content (slides, code, diagrams). Returns structured summaries with visual artifacts for AI agents to work with.

## What it does

Existing YouTube analysis tools work only with subtitles. This server fills the gap: it analyzes both the **audio track** (transcript with timestamps) and the **video track** (key frames), producing structured output that any MCP-compatible AI agent can use.

**Input:** YouTube URL
**Output:** Time-based chunks with compressed summaries + visual artifacts (images, OCR text)

## How it works

```
YouTube URL
    ↓
Download (yt-dlp) → video + audio + subtitles
    ↓
┌────────────────────┐  ┌──────────────────────────────────┐
│ Text branch        │  │ Visual branch                    │
│                    │  │                                  │
│ Subtitles/Whisper  │  │ FFmpeg frames → pHash dedup      │
│     ↓              │  │     ↓                            │
│ 30-sec chunks      │  │ CLIP classify (filter talking    │
│     ↓              │  │   head) → OCR for slides/code   │
│ Qwen3 8B summary   │  │     ↓                            │
│ (speaker's voice,  │  │ Useful artifacts with timestamps │
│  5x compression)   │  │                                  │
└────────┬───────────┘  └──────────┬───────────────────────┘
         │                        │
         └────── Match by time ───┘
                     ↓
            Result: summaries + artifacts
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `add_video` | Submit YouTube URL for analysis. Returns `task_id` instantly. |
| `get_status` | Poll task progress: downloading → transcribing → processing_frames → summarizing → completed |
| `get_result` | Get text summaries + artifact metadata (lightweight, no images) |
| `get_artifacts` | Get images for a specific chunk by index (for deep dive) |
| `delete_task` | Remove task and all artifacts |
| `retry_task` | Re-run a failed task from last checkpoint (skips completed steps) |

**Typical flow:**
1. `add_video(url)` → get `task_id`
2. Poll `get_status(task_id)` until `completed`
3. `get_result(task_id)` → read summaries, identify chunks of interest
4. `get_artifacts(task_id, chunk_index=5)` → view images for that chunk

## Quick Start

### Prerequisites

- Docker + Docker Compose
- [Ollama](https://ollama.com/) installed natively (for GPU-accelerated summarization)

### 1. Clone and build

```bash
git clone git@github.com:um4ru-ch4n/youtube-analyzer-mcp.git
cd youtube-analyzer-mcp
```

### 2. Start Ollama and pull the model

```bash
# Start Ollama (if not already running)
brew services start ollama   # macOS
# or: ollama serve            # manual start

# Pull the summarization model (~5 GB)
ollama pull qwen3:8b
```

### 3. Start the MCP server and sidecars

```bash
docker-compose up -d mcp-server whisper clip
```

This starts:
- **mcp-server** (port 39280) — Go MCP server + yt-dlp + ffmpeg + tesseract
- **whisper** — faster-whisper sidecar (fallback when no subtitles available)
- **clip** — CLIP sidecar for frame classification

Ollama runs natively on your host (not in Docker) for GPU acceleration. The MCP server reaches it via `host.docker.internal:11434`.

### 4. Connect to Claude Code

Add to `~/.claude.json`:

```json
{
  "mcpServers": {
    "youtube-analyzer": {
      "type": "http",
      "url": "http://localhost:39280/mcp"
    }
  }
}
```

Restart Claude Code. The `youtube-analyzer` server should appear in `/mcp`.

### 5. Test it

In Claude Code, say:
> Analyze this video: https://youtu.be/SmYNK0kqaDI

Claude will call `add_video`, poll `get_status`, then `get_result` to show you the analysis.

## Configuration

### config.yaml

Key settings (all configurable):

```yaml
pipeline:
  chunk_duration_sec: 30          # chunk length for summarization
  summary_compression_ratio: 5    # compress transcript 5x
  fallback_interval_sec: 5        # frame extraction interval (seconds)
  max_video_duration_sec: 7200    # max 2 hours
  dedup_similarity_threshold: 0.95 # pHash dedup threshold

summarizer:
  ollama:
    url: "http://host.docker.internal:11434"
    model: "qwen3:8b"
```

### Environment Variables

```bash
GEMINI_API_KEY=...       # Optional: for Vision analysis of diagram frames
ANTHROPIC_API_KEY=...    # Optional: alternative Vision provider (Claude)
```

Without Vision API keys, diagram frames are kept as-is (no filtering) and returned as images for Claude to analyze directly.

## Data Storage

```
data/
├── youtube-analyzer.db           # SQLite: task tracking
└── tasks/{task_id}/
    ├── video.mp4                 # Downloaded video
    ├── audio.wav                 # Audio track
    ├── subs.en.vtt               # Subtitles (if available)
    ├── frames/                   # Extracted frames
    ├── processing/               # Intermediate results (JSON)
    │   ├── transcript.json
    │   ├── frames_deduped.json
    │   ├── frames_classified.json
    │   ├── chunks.json
    │   ├── summaries.json
    │   └── result.json
    └── checkpoint.json           # Resume point for retry
```

## Development

```bash
# Build
go build ./cmd/server

# Run tests
go test ./...

# Run locally (stdio mode)
go run ./cmd/server

# Docker
docker-compose up --build           # all services
docker-compose up mcp-server -d     # just MCP server
docker-compose logs -f mcp-server   # follow logs
```

## Tech Stack

- **Go** — MCP server, pipeline orchestration
- **mcp-go** (mark3labs) — MCP protocol
- **SQLite** (modernc.org/sqlite, pure Go) — task tracking
- **yt-dlp** — video/audio/subtitle download
- **FFmpeg** — frame extraction
- **faster-whisper** — speech-to-text (Python sidecar)
- **CLIP** — frame classification (Python sidecar)
- **Tesseract** — OCR for slides/code
- **Ollama + Qwen3 8B** — text summarization (native, GPU)
- **Gemini/Claude API** — optional Vision analysis for diagrams

## VPN Split Tunneling

If you use a VPN with split tunneling, add these domains to route through VPN:

```
# YouTube video download (yt-dlp)
youtube.com
*.youtube.com
youtu.be
*.googlevideo.com
*.ytimg.com
*.ggpht.com

# Google API (Gemini Vision)
*.googleapis.com

# Docker Hub (image pull)
registry-1.docker.io
auth.docker.io
production.cloudflare.docker.com
```

Without these, yt-dlp will fail with SSL handshake timeouts when downloading videos inside Docker containers.

## License

MIT
