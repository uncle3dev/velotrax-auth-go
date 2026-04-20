package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/uncle3dev/velotrax-auth-go/internal/config"
	"github.com/uncle3dev/velotrax-auth-go/internal/db"
	"github.com/uncle3dev/velotrax-auth-go/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger, err := initLogger(cfg.LogLevel)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting velotrax-auth-go",
		zap.String("env", cfg.AppEnv),
		zap.Int("port", cfg.AppPort),
	)

	mongoDB, err := db.Connect(context.Background(), cfg.MongoURI)
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mongoDB.Disconnect(shutdownCtx)
	}()
	logger.Info("Connected to MongoDB")

	if err := db.EnsureIndexes(context.Background(), mongoDB.Database); err != nil {
		logger.Fatal("Failed to ensure indexes", zap.Error(err))
	}

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	router.Setup(engine, mongoDB, logger, cfg)

	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.AppPort),
		Handler:        engine,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		logger.Info("Server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	logger.Info("Server stopped")
}

func initLogger(level string) (*zap.Logger, error) {
	var cfg zap.Config
	if level == "debug" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}
	cfg.Level = zap.NewAtomicLevelAt(parseLogLevel(level))
	return cfg.Build()
}

func parseLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zap.DebugLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}
