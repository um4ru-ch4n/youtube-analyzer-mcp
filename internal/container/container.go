package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/adapter/clip"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/adapter/ffmpeg"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/adapter/ocr"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/adapter/phash"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/adapter/summarizer"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/adapter/vision"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/adapter/whisper"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/adapter/ytdlp"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/config"
	mcphandler "github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/handler/mcp"
	taskrepo "github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/repository/task"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/service/task"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/usecase/pipeline"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

// Container holds all application dependencies and provides a single point
// for initialization and graceful teardown.
type Container struct {
	TaskRepo    *taskrepo.Repository
	TaskManager *task.Manager
	Handler     *mcphandler.Handler

	// Adapters (stored for potential future access / testing).
	Downloader     *ytdlp.Downloader
	Extractor      *ffmpeg.Extractor
	Transcriber    *whisper.Transcriber
	Classifier     *clip.Classifier
	OCRReader      *ocr.Reader
	Deduplicator   *phash.Deduplicator
	VisionAnalyzer pipeline.VisionAnalyzer
	Summarizer     *summarizer.OllamaSummarizer
}

// New creates the full dependency graph from configuration.
func New(cfg *config.Config, env *config.EnvConfig) (*Container, error) {
	zapLogger := logger.Logger().Desugar()

	// 1. Ensure data directory exists and open SQLite.
	dbDir := cfg.Storage.DataDir
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir %s: %w", dbDir, err)
	}

	dbPath := filepath.Join(dbDir, "youtube-analyzer.db")
	repo, err := taskrepo.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open task repository: %w", err)
	}

	// 2. Create adapters.
	downloader := ytdlp.New(zapLogger)

	extractor := ffmpeg.New(ffmpeg.Config{
		SceneThreshold:   cfg.Pipeline.SceneDetectThreshold,
		MinFrames:        cfg.Pipeline.SceneDetectMinFrames,
		MaxFrames:        cfg.Pipeline.SceneDetectMaxFrames,
		FallbackInterval: cfg.Pipeline.FallbackIntervalSec,
	}, zapLogger)

	transcriber := whisper.New(cfg.Whisper.URL, cfg.Whisper.TimeoutSec)

	classifier := clip.New(cfg.Clip.URL, cfg.Clip.TimeoutSec)

	ocrReader := ocr.New(cfg.OCR.Languages, zapLogger)

	deduplicator := phash.New(cfg.Pipeline.DedupSimilarityThreshold, zapLogger)

	visionAnalyzer := createVisionAnalyzer(cfg, env, zapLogger)

	ollamaSummarizer := summarizer.New(
		cfg.Summarizer.Ollama.URL,
		cfg.Summarizer.Ollama.Model,
		cfg.Summarizer.Ollama.TimeoutSec,
	)

	// 3. Create pipeline runner.
	pipelineRunner := pipeline.New(
		downloader,
		transcriber,
		extractor,
		classifier,
		ocrReader,
		visionAnalyzer,
		ollamaSummarizer,
		deduplicator,
		repo, // StatusUpdater — repo implements UpdateStatus + UpdateWarnings
		cfg.Pipeline,
		cfg.Storage.DataDir,
	)

	// 4. Create task manager.
	manager := task.New(repo, pipelineRunner, cfg.WorkerPool.Size, cfg.Storage.DataDir)

	// 5. Create MCP handler.
	handler := mcphandler.New(manager)

	return &Container{
		TaskRepo:       repo,
		TaskManager:    manager,
		Handler:        handler,
		Downloader:     downloader,
		Extractor:      extractor,
		Transcriber:    transcriber,
		Classifier:     classifier,
		OCRReader:      ocrReader,
		Deduplicator:   deduplicator,
		VisionAnalyzer: visionAnalyzer,
		Summarizer:     ollamaSummarizer,
	}, nil
}

// Close releases all resources held by the container.
func (c *Container) Close() error {
	return c.TaskRepo.Close()
}

// createVisionAnalyzer selects the vision provider based on configuration.
func createVisionAnalyzer(cfg *config.Config, env *config.EnvConfig, zapLogger *zap.Logger) pipeline.VisionAnalyzer {
	provider := strings.ToLower(cfg.Vision.Provider)

	if provider == "claude" {
		zapLogger.Info("using Claude as vision provider")
		return vision.NewClaude(
			env.AnthropicKey,
			cfg.Vision.Claude.Model,
			cfg.Vision.Claude.TimeoutSec,
		)
	}

	zapLogger.Info("using Gemini as vision provider")
	return vision.NewGemini(
		env.GeminiAPIKey,
		cfg.Vision.Gemini.Model,
		cfg.Vision.Gemini.TimeoutSec,
	)
}
