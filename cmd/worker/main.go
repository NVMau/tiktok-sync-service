package main

import (
	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/logger"
	"github.com/user/sync-tiktok-mps/internal/workers"
)

func main() {
	cfg := config.Load()

	// Initialize logger
	logger.Init(&logger.Config{
		Level:       cfg.LogLevel,
		Environment: cfg.Environment,
		OutputPath:  cfg.LogOutput,
	})
	defer logger.Sync()

	log := logger.Log.Named("worker")

	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisURL},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)

	mux := asynq.NewServeMux()
	workers.RegisterHandlers(mux)

	log.Info("starting Asynq worker",
		zap.String("redis", cfg.RedisURL),
		zap.String("environment", cfg.Environment))

	if err := srv.Run(mux); err != nil {
		log.Fatal("failed to start worker", zap.Error(err))
	}
}
