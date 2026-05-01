package main

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap/zapcore"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/app"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/config"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	env := config.LoadEnv()

	level := parseLogLevel(env.LogLevel)
	if env.IsDevelopment() {
		logger.SetLogger(logger.NewDev(level))
	}
	if !env.IsDevelopment() {
		logger.SetLogger(logger.New(level))
	}

	ctx := context.Background()
	logger.InfoKV(ctx, "starting youtube-analyzer-mcp server",
		"version", "0.1.0",
		"transport", env.TransportMode,
		"log_level", env.LogLevel,
		"app_env", env.AppEnv,
	)

	app.Run(cfg, env)
}

func parseLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
