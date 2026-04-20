package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/uncle3dev/velotrax-auth-go/internal/config"
	"github.com/uncle3dev/velotrax-auth-go/internal/db"
	"github.com/uncle3dev/velotrax-auth-go/internal/middleware"
)

func Setup(engine *gin.Engine, mongoDB *db.DB, logger *zap.Logger, cfg *config.Config) {
	engine.Use(middleware.Logger(logger))
	engine.Use(middleware.Recovery(logger))
	engine.Use(middleware.CORS())

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// TODO: register auth handlers here
	_ = mongoDB
	_ = cfg
}
