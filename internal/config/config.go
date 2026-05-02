package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Pipeline   PipelineConfig   `mapstructure:"pipeline"`
	WorkerPool WorkerPoolConfig `mapstructure:"worker_pool"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Whisper    WhisperConfig    `mapstructure:"whisper"`
	Clip       ClipConfig       `mapstructure:"clip"`
	Vision     VisionConfig     `mapstructure:"vision"`
	Summarizer SummarizerConfig `mapstructure:"summarizer"`
	OCR        OCRConfig        `mapstructure:"ocr"`
	HTTP       HTTPConfig       `mapstructure:"http"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

type PipelineConfig struct {
	ChunkDurationSec         int     `mapstructure:"chunk_duration_sec"`
	OverlapSec               int     `mapstructure:"overlap_sec"`
	SummaryCompressionRatio  int     `mapstructure:"summary_compression_ratio"`
	MaxVideoDurationSec      int     `mapstructure:"max_video_duration_sec"`
	MaxConcurrentFrames      int     `mapstructure:"max_concurrent_frames"`
	SceneDetectThreshold     float64 `mapstructure:"scene_detect_threshold"`
	SceneDetectMinFrames     int     `mapstructure:"scene_detect_min_frames"`
	SceneDetectMaxFrames     int     `mapstructure:"scene_detect_max_frames"`
	FallbackIntervalSec      int     `mapstructure:"fallback_interval_sec"`
	DedupSimilarityThreshold float64 `mapstructure:"dedup_similarity_threshold"`
}

type WorkerPoolConfig struct {
	Size      int `mapstructure:"size"`
	QueueSize int `mapstructure:"queue_size"`
}

type StorageConfig struct {
	DataDir string `mapstructure:"data_dir"`
}

type WhisperConfig struct {
	URL        string `mapstructure:"url"`
	TimeoutSec int    `mapstructure:"timeout_sec"`
	Model      string `mapstructure:"model"`
}

type ClipConfig struct {
	URL        string `mapstructure:"url"`
	TimeoutSec int    `mapstructure:"timeout_sec"`
}

type VisionConfig struct {
	Provider string       `mapstructure:"provider"`
	Gemini   GeminiConfig `mapstructure:"gemini"`
	Claude   ClaudeConfig `mapstructure:"claude"`
}

type GeminiConfig struct {
	Model      string `mapstructure:"model"`
	TimeoutSec int    `mapstructure:"timeout_sec"`
}

type ClaudeConfig struct {
	Model      string `mapstructure:"model"`
	TimeoutSec int    `mapstructure:"timeout_sec"`
}

type SummarizerConfig struct {
	Provider string       `mapstructure:"provider"`
	Ollama   OllamaConfig `mapstructure:"ollama"`
}

type OllamaConfig struct {
	URL        string `mapstructure:"url"`
	Model      string `mapstructure:"model"`
	TimeoutSec int    `mapstructure:"timeout_sec"`
}

type OCRConfig struct {
	Languages []string `mapstructure:"languages"`
}

type HTTPConfig struct {
	Port int    `mapstructure:"port"`
	Path string `mapstructure:"path"`
}

type LoggingConfig struct {
	DefaultLevel string `mapstructure:"default_level"`
}

type EnvConfig struct {
	TransportMode string
	HTTPPort      int
	LogLevel      string
	AppEnv        string
	GeminiAPIKey  string
	AnthropicKey  string
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/app")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}

func LoadEnv() *EnvConfig {
	return &EnvConfig{
		TransportMode: getEnvDefault("TRANSPORT_MODE", "stdio"),
		HTTPPort:      getEnvInt("HTTP_PORT", 3000),
		LogLevel:      getEnvDefault("LOG_LEVEL", "info"),
		AppEnv:        getEnvDefault("APP_ENV", "production"),
		GeminiAPIKey:  os.Getenv("GEMINI_API_KEY"),
		AnthropicKey:  os.Getenv("ANTHROPIC_API_KEY"),
	}
}

func (e *EnvConfig) IsDevelopment() bool {
	return strings.ToLower(e.AppEnv) == "development"
}

func (e *EnvConfig) IsHTTPMode() bool {
	return strings.ToLower(e.TransportMode) == "http"
}

func getEnvDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	var result int
	_, _ = fmt.Sscanf(v, "%d", &result)
	if result == 0 {
		return defaultVal
	}
	return result
}
