// Command server 是 TrainTrainSkill 的 HTTP 服务入口。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rrjLoki/TrainTrainSkill/internal/config"
	"github.com/rrjLoki/TrainTrainSkill/internal/httpapi"
	"github.com/rrjLoki/TrainTrainSkill/internal/llm"
	"github.com/rrjLoki/TrainTrainSkill/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: httpapi.New(httpapi.Deps{
			Store:  store.New(),
			LLM:    llm.NewClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel),
			Logger: logger,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		// 不设 WriteTimeout：/api/chat/stream 是长连接，超时由客户端 context 决定。
		IdleTimeout: 60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("http server listening",
			"addr", cfg.Addr,
			"llm_base_url", cfg.LLMBaseURL,
			"llm_model", cfg.LLMModel,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received, draining connections")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
