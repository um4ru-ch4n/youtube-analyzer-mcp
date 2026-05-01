# YouTube Video Analysis MCP Server — Architecture & Implementation Plan

## Context

Существующие инструменты для работы с YouTube (NoteGPT, Glasp, Eightify) работают только с субтитрами. Для технического контента (туториалы, лекции, демо) значительная часть информации — визуальная: код в редакторе, диаграммы, слайды. Нужен MCP-сервер, который анализирует и аудио (транскрипт), и видео (ключевые фреймы), и возвращает AI-агенту полный контекст.

**Проект:** `youtube-analyzer-mcp` на GitHub (um4ru-ch4n).
**Путь:** `/Users/apoleynikov/Main/Programming/Go/src/github.com/um4ru-ch4n/youtube-analyzer-mcp`

---

## Ключевые улучшения ТЗ

### 1. CLIP и Whisper — sidecar-контейнеры, не subprocess
Отдельные Python sidecar-контейнеры (FastAPI), модели загружаются один раз при старте. Go общается по HTTP через shared volume.

### 2. Chunking по целевой длительности (~45 сек), не по N сегментов
Сегменты Whisper имеют случайную длину. Chunking по времени даёт предсказуемые чанки.

### 3. Два отдельных провайдера: Vision + Summarizer
- **VisionAnalyzer** — анализ изображений (Gemini/Claude API). Только для диаграмм и `other`.
- **Summarizer** — текстовый саммари чанков. Локальная модель через **Ollama sidecar** (Qwen3 8B — лучшее качество среди 7B моделей, отличная мультиязычность EN+RU). Бесплатно, интерфейс готов к замене на свой сервер. Модель переключается одной строкой конфига.

### 4. Подробное логирование ошибок
Каждая ошибка логируется со структурированным контекстом: step name, task_id, frame index, входные данные, причина. Non-fatal (OCR failure на одном фрейме) → лог warning + добавление в `task.warnings[]`. Fatal (download failed) → лог error + `task.status = failed` + `task.error` с полным описанием. Нейросеть при `get_status` получает статус и описание ошибки, сама решает что делать.

### 5. pHash дедупликация ДО CLIP
Экономим на классификации. Порог similarity конфигурируем (дефолт 0.95). Последовательные фреймы с similarity > threshold → оставляем только первый.

### 6. MAX_VIDEO_DURATION = 2 часа (конфигурируемый)

### 7. Overlap как контекст, не как контент
Overlap включается в промпт LLM для связности, но не в финальный вывод чанка.

### 8. Scene detection — конфигурируемый порог + fallback
Если scene detection даёт слишком мало/много фреймов — fallback на регулярный интервал (каждые N секунд).

### 9. Tool `delete_task` — очистка завершённых задач
Удаление задачи и всех артефактов (папка + запись в SQLite) по task_id. Только для задач со статусом `completed` или `failed`.

---

## Архитектурные решения

### MCP server initialization
MCP-сервер инициализируется в `main.go` (или `app.go`), **не** в handler. Handler — это transport layer: преобразование MCP request → внутренние DTO → вызов service → подготовка response.

### Интерфейсы — по месту использования (Go best practice)
Интерфейсы определяются в пакете-потребителе, не в пакете-реализации. Допустимо: файл `deps.go` в пакете с зависимостями интерфейсными (как в marketplace-promocode). Глобальные интерфейсы только для cross-cutting concerns (logger).

### Transport
stdio + HTTP (как buildin-mcp-v2). Async-паттерн (add_video → get_status → get_result) обходит таймауты MCP.

### ML-сервисы (все — sidecar-контейнеры в Docker Compose)
- **whisper** — FastAPI + faster-whisper (large-v3-turbo)
- **clip** — FastAPI + CLIP
- **ollama** — Ollama с моделью для summarization (Llama 3.1 8B)

Go-сервер содержит: yt-dlp, ffmpeg, tesseract.

### Task storage (гибридный)
- **SQLite** (`modernc.org/sqlite`, pure Go) — task tracking: статусы, прогресс, метаданные, warnings, result text
- **Файловая система** `data/tasks/{task_id}/` — артефакты: аудио, видео, фреймы, result.json

### MCP Library
`github.com/mark3labs/mcp-go` (та же, что в buildin-mcp-v2).

---

## Структура проекта

```
youtube-analyzer-mcp/
├── cmd/server/main.go                    # Entry point: config, logger, MCP init, transport
├── internal/
│   ├── app/app.go                        # Application: MCP server creation, wiring tools, lifecycle
│   ├── config/config.go                  # Viper config.yaml + EnvConfig
│   ├── handler/mcp/                      # Transport layer (MCP request → DTO → service → response)
│   │   ├── add_video.go
│   │   ├── get_status.go
│   │   ├── get_result.go
│   │   ├── delete_task.go
│   │   └── helpers.go
│   ├── service/
│   │   ├── task/                         # TaskManager: queue + worker pool
│   │   │   ├── manager.go
│   │   │   ├── worker.go
│   │   │   └── deps.go                  # Interfaces used by task service
│   │   └── chunking/
│   │       ├── chunker.go               # Time-based chunking + alignment
│   │       └── deps.go
│   ├── usecase/pipeline/
│   │   ├── runner.go                     # Orchestrator (parallel fan-out via errgroup)
│   │   ├── state.go                      # PipelineState (passed between steps)
│   │   ├── deps.go                       # Step interfaces + adapter deps
│   │   ├── step_download.go
│   │   ├── step_transcribe.go
│   │   ├── step_extract_frames.go
│   │   ├── step_process_frames.go
│   │   ├── step_chunk.go
│   │   ├── step_summarize.go
│   │   └── step_result.go
│   ├── adapter/                          # External tool implementations
│   │   ├── ytdlp/downloader.go           # yt-dlp subprocess
│   │   ├── whisper/transcriber.go        # HTTP client to whisper sidecar
│   │   ├── ffmpeg/extractor.go           # Frame extraction subprocess
│   │   ├── clip/classifier.go            # HTTP client to CLIP sidecar
│   │   ├── ocr/reader.go                 # Tesseract subprocess
│   │   ├── phash/deduplicator.go         # Perceptual hash dedup
│   │   ├── vision/                       # Vision adapter (image analysis)
│   │   │   ├── gemini.go
│   │   │   └── claude.go
│   │   └── summarizer/                   # Summarizer adapter (text summaries)
│   │       └── ollama.go                 # Ollama HTTP client (local LLM)
│   ├── model/                            # Domain models
│   │   ├── task.go                       # Task, TaskStatus, TaskResult, Warning
│   │   ├── video.go                      # VideoMeta, DownloadResult
│   │   ├── transcript.go                # TranscriptSegment, Transcript
│   │   ├── frame.go                      # Frame, FrameType, FrameClassification, FrameAnalysis
│   │   ├── chunk.go                      # Chunk, ChunkSummary
│   │   └── errors.go                     # Domain errors
│   ├── repository/task/
│   │   ├── sqlite.go                     # SQLite implementation (modernc.org/sqlite)
│   │   └── migrations.go                # Schema DDL
│   └── container/container.go            # DI wiring (all adapters, services, pipeline)
├── sidecar/
│   ├── whisper/                          # FastAPI + faster-whisper
│   │   ├── main.py
│   │   ├── Dockerfile
│   │   └── requirements.txt
│   └── clip/                             # FastAPI + CLIP
│       ├── main.py
│       ├── Dockerfile
│       └── requirements.txt
├── pkg/logger/                           # Zap structured logger (from buildin-mcp-v2)
│   ├── logger.go
│   └── context.go
├── config.yaml                           # Non-secret config
├── .env.example                          # Environment variables template
├── Dockerfile                            # Multi-stage: Go build + runtime (ffmpeg, tesseract, yt-dlp)
├── docker-compose.yml                    # Go server + whisper + clip + ollama
├── Makefile                              # build, test, lint, docker-up, docker-down
├── CLAUDE.md
└── README.md

Файловая структура артефактов:
data/
├── youtube-analyzer.db                   # SQLite: tasks, statuses, metadata, result text
└── tasks/
    └── {task_id}/                        # Папка на каждую задачу
        ├── audio.wav                     # Скачанное аудио
        ├── video.mp4                     # Скачанное видео
        ├── frames/                       # Извлечённые фреймы
        │   ├── frame_001_45.2s.jpg
        │   └── ...
        └── result.json                   # Финальный результат
```

---

## Ключевые интерфейсы (по месту использования)

Интерфейсы лежат в `deps.go` в пакете-потребителе, не в пакете-реализации.

```go
// internal/usecase/pipeline/deps.go — интерфейсы, от которых зависит pipeline
type Downloader interface {
    Download(ctx context.Context, url, outputDir string) (model.DownloadResult, error)
}
type Transcriber interface {
    Transcribe(ctx context.Context, audioPath string) (model.Transcript, error)
}
type FrameExtractor interface {
    ExtractFrames(ctx context.Context, videoPath, outputDir string) ([]model.Frame, error)
}
type FrameClassifier interface {
    Classify(ctx context.Context, imagePath string) (model.FrameClassification, error)
    ClassifyBatch(ctx context.Context, paths []string) ([]model.FrameClassification, error)
}
type OCRReader interface {
    ReadText(ctx context.Context, imagePath string) (string, error)
}
type VisionAnalyzer interface {
    AnalyzeImage(ctx context.Context, imagePath, prompt string) (string, error)
}
type Summarizer interface {
    SummarizeChunk(ctx context.Context, transcriptText string, frameContents []model.FrameContent) (string, error)
}
type Deduplicator interface {
    FilterDuplicates(ctx context.Context, frames []model.Frame) ([]model.Frame, error)
}

// internal/service/task/deps.go — интерфейсы, от которых зависит task manager
type TaskRepository interface {
    Create(ctx context.Context, task model.Task) error
    Get(ctx context.Context, taskID string) (model.Task, error)
    UpdateStatus(ctx context.Context, taskID string, status model.TaskStatusEnum, progress string) error
    UpdateWarnings(ctx context.Context, taskID string, warnings []model.Warning) error
    SaveResult(ctx context.Context, taskID string, result model.TaskResult) error
    Delete(ctx context.Context, taskID string) error
    List(ctx context.Context, limit, offset int) ([]model.Task, error)
}

type PipelineRunner interface {
    Run(ctx context.Context, task model.Task) (model.TaskResult, error)
}
```

---

## MCP Tools (4 инструмента)

| Tool | Input | Output | Описание |
|------|-------|--------|----------|
| `add_video` | `{url: string}` | `{task_id: string}` | Добавить видео в очередь. Возвращает task_id мгновенно. |
| `get_status` | `{task_id: string}` | `{task_id, status, progress, error?, warnings?}` | Текущий статус задачи. |
| `get_result` | `{task_id: string}` | `{task_id, video_url, title, duration, chunks[]}` | Результат (только для completed). |
| `delete_task` | `{task_id: string}` | `{success: bool}` | Удалить задачу и артефакты (completed/failed). |

---

## Pipeline Flow

```
add_video(url) → task_id (instant)
    ↓ (worker picks up)
DownloadStep: yt-dlp → video + audio files + metadata
    ↓ (parallel fan-out via errgroup)
    ├── TranscribeStep: whisper HTTP → []TranscriptSegment
    └── FrameExtractStep:
    │     ffmpeg (scene detect + fallback interval)
    │     → phash dedup (before CLIP)
    │     → CLIP classify (talking_head/slide/code/diagram/other)
    │     → OCR (slide, code) / Vision API (diagram, other)
    ↓ (join)
ChunkStep: merge transcript + processed frames by time windows (~45sec)
    ↓
SummarizeStep: Ollama (local LLM) → summary per chunk
    ↓
ResultStep: assemble TaskResult, save to SQLite + result.json, cleanup tmp files
```

---

## Docker Compose

```yaml
services:
  mcp-server:        # Go MCP server + ffmpeg + tesseract + yt-dlp
  whisper:           # FastAPI + faster-whisper (large-v3-turbo)
  clip:              # FastAPI + CLIP
  ollama:            # Ollama with Qwen3 8B (summarization)

volumes:
  shared-data:       # Shared volume for audio/video/frames between services
  ollama-models:     # Persistent volume for Ollama model cache
```

---

## Логирование и обработка ошибок

Структурированное логирование через zap с контекстом:
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

- **Non-fatal**: лог warning → добавление в `task.warnings[]` → продолжение pipeline
- **Fatal**: лог error → `task.status = "failed"` → `task.error = "Download failed: video is private"` → cleanup tmp
- Нейросеть при `get_status` видит warnings и error, сама решает что делать

---

## Порядок реализации

### Phase 1: Скелет
1. Создать GitHub repo `um4ru-ch4n/youtube-analyzer-mcp`
2. `cmd/server/main.go` — entry point (config, logger, transport)
3. `internal/app/app.go` — MCP server creation, tool registration
4. `internal/config/` — viper config.yaml + EnvConfig
5. `pkg/logger/` — zap structured logger
6. `internal/model/` — все доменные типы
7. `internal/handler/mcp/` — transport layer handlers (stubs)
8. Проверить: MCP сервер стартует через stdio, tools отвечают

### Phase 2: Task Infrastructure
9. `internal/repository/task/sqlite.go` — SQLite (modernc.org/sqlite)
10. `internal/service/task/manager.go` — очередь + worker pool
11. Подключить handlers к TaskManager
12. Тесты lifecycle задач

### Phase 3: Адаптеры
13. `internal/adapter/ytdlp/` — subprocess wrapper
14. `internal/adapter/ffmpeg/` — frame extraction (scene detect + interval fallback)
15. `internal/adapter/whisper/` — HTTP client to sidecar
16. `internal/adapter/clip/` — HTTP client to sidecar
17. `internal/adapter/ocr/` — tesseract subprocess
18. `internal/adapter/phash/` — perceptual hash dedup
19. `internal/adapter/vision/` — Gemini + Claude
20. `internal/adapter/summarizer/` — Ollama HTTP client
21. Тесты адаптеров

### Phase 4: Pipeline
22. `internal/usecase/pipeline/` — step interface + all steps
23. PipelineRunner с parallel fan-out (errgroup)
24. Chunking service
25. Integration test с mock-адаптерами

### Phase 5: DI + Docker
26. `internal/container/container.go` — wire everything
27. `sidecar/whisper/` — FastAPI + Dockerfile
28. `sidecar/clip/` — FastAPI + Dockerfile
29. Main Dockerfile
30. `docker-compose.yml` (+ ollama service)

### Phase 6: Polish
31. CLAUDE.md, README.md, Makefile, .env.example
32. Edge cases: длинные видео, приватные, no audio
33. delete_task tool implementation

---

## Verification

1. **Unit tests:** `go test ./...` — адаптеры, pipeline steps, task manager, handlers
2. **Manual MCP:** stdio → add_video → get_status (polling) → get_result → delete_task
3. **Docker:** `docker-compose up` → HTTP transport → полный цикл
4. **Edge cases:** видео >2h (отказ), без речи (пустой транскрипт), скринкаст (много code-фреймов), приватное видео (ошибка download)

---

## Файлы-шаблоны из референсных проектов

| Файл | Что берём |
|------|-----------|
| `buildin-mcp-v2/cmd/server/main.go` | Entry point, transport selection, config/logger init |
| `buildin-mcp-v2/internal/service/mcp/tools/registry.go` | Tool registration pattern |
| `buildin-mcp-v2/internal/config/config.go` | Viper config + EnvConfig |
| `buildin-mcp-v2/internal/adapter/buildin/client.go` | HTTP client with retry pattern |
| `buildin-mcp-v2/pkg/logger/` | Zap logger package |
| `service-marketplace-promocode/internal/container/` | DI container pattern |
| `service-marketplace-promocode/internal/usecases/` | Usecase layer + deps.go pattern |
