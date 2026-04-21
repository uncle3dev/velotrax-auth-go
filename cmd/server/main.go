package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"

	"github.com/uncle3dev/velotrax-auth-go/internal/config"
	"github.com/uncle3dev/velotrax-auth-go/internal/interceptor"
	"github.com/uncle3dev/velotrax-auth-go/internal/db"
	authpb "github.com/uncle3dev/velotrax-auth-go/internal/gen/auth"
	"github.com/uncle3dev/velotrax-auth-go/internal/middleware"
	"github.com/uncle3dev/velotrax-auth-go/internal/router"
	"github.com/uncle3dev/velotrax-auth-go/internal/service"
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

	mongoDB, err := db.Connect(context.Background(), cfg.MongoURI)
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mongoDB.Disconnect(shutdownCtx)
	}()

	if err := db.EnsureIndexes(context.Background(), mongoDB.Database); err != nil {
		logger.Fatal("Failed to ensure indexes", zap.Error(err))
	}

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// gRPC server
	grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(interceptor.UnaryLogger(logger)))
	authpb.RegisterAuthServiceServer(grpcSrv, service.NewAuthService(mongoDB.Database, cfg, logger))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		logger.Fatal("Failed to listen for gRPC", zap.Error(err))
	}

	go func() {
		logger.Info("gRPC server listening", zap.Int("port", cfg.GRPCPort))
		if err := grpcSrv.Serve(lis); err != nil {
			logger.Fatal("gRPC server error", zap.Error(err))
		}
	}()

	// HTTP server (health check)
	engine := gin.New()
	engine.Use(middleware.Logger(logger), middleware.Recovery(logger), middleware.CORS())
	router.Setup(engine, mongoDB, logger, cfg)

	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: engine,
	}

	go func() {
		logger.Info("HTTP server listening", zap.Int("port", cfg.HTTPPort))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	grpcSrv.GracefulStop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpSrv.Shutdown(ctx)
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
