package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/server"

	"github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/config"
	mcphandler "github.com/um4ru-ch4n/youtube-analyzer-mcp/internal/handler/mcp"
	"github.com/um4ru-ch4n/youtube-analyzer-mcp/pkg/logger"
)

const (
	serverName    = "youtube-analyzer-mcp"
	serverVersion = "0.1.0"
)

func Run(cfg *config.Config, env *config.EnvConfig) {
	ctx := context.Background()

	s := newMCPServer(cfg, env)

	if env.IsHTTPMode() {
		runHTTP(ctx, s, cfg, env)
		return
	}

	runStdio(ctx, s)
}

func newMCPServer(cfg *config.Config, env *config.EnvConfig) *server.MCPServer {
	s := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(false),
	)

	registerTools(s, cfg, env)

	return s
}

func registerTools(s *server.MCPServer, _ *config.Config, _ *config.EnvConfig) {
	// TODO: replace nil with real TaskService once service/task is implemented
	var taskService mcphandler.TaskService

	handler := mcphandler.New(taskService)
	handler.RegisterTools(s)
}

func runStdio(_ context.Context, s *server.MCPServer) {
	logger.Info(context.Background(), "starting stdio transport")

	if err := server.ServeStdio(s); err != nil {
		logger.Fatalf(context.Background(), "stdio server error: %v", err)
	}
}

func runHTTP(ctx context.Context, s *server.MCPServer, cfg *config.Config, env *config.EnvConfig) {
	port := env.HTTPPort
	path := cfg.HTTP.Path

	mux := http.NewServeMux()

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			writeJSONError(w, -32000, "Method not allowed")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSONError(w, -32700, "Failed to read request body")
			return
		}
		defer r.Body.Close()

		response := s.HandleMessage(r.Context(), body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(response)
	})

	addr := fmt.Sprintf(":%d", port)
	logger.InfoKV(ctx, "HTTP server starting", "port", port, "path", path)

	httpServer := &http.Server{Addr: addr, Handler: mux}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info(ctx, "shutting down HTTP server")
		_ = httpServer.Close()
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf(ctx, "HTTP server error: %v", err)
	}

	logger.Info(ctx, "HTTP server stopped")
}

func writeJSONError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"error":   map[string]any{"code": code, "message": message},
		"id":      nil,
	})
}

