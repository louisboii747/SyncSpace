// Package api exposes SyncSpace application services over HTTP.
package api

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/louisboii747/syncspace/backend/internal/models"
)

// DiscoveryService is the API-facing discovery application contract.
type DiscoveryService interface {
	Devices() []models.Device
	Self() models.Device
	Refresh()
}

// RouterConfig supplies HTTP dependencies.
type RouterConfig struct {
	Discovery       DiscoveryService
	DiscoverySocket gin.HandlerFunc
	Pairing         PairingService
	PairingSocket   gin.HandlerFunc
	Logger          *slog.Logger
}

// NewRouter creates the SyncSpace HTTP router.
func NewRouter(config RouterConfig) *gin.Engine {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(requestLogger(logger), recovery(logger))

	router.GET("/devices", func(c *gin.Context) {
		c.JSON(http.StatusOK, config.Discovery.Devices())
	})
	router.GET("/device/self", func(c *gin.Context) {
		c.JSON(http.StatusOK, config.Discovery.Self())
	})
	router.POST("/discovery/refresh", func(c *gin.Context) {
		config.Discovery.Refresh()
		c.JSON(http.StatusAccepted, gin.H{"status": "refresh_requested"})
	})
	router.GET("/ws/discovery", config.DiscoverySocket)
	registerPairingRoutes(router, config.Pairing, config.PairingSocket, logger)
	return router
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		logger.Info("HTTP request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(started).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

func recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("HTTP panic", "error", recovered, "stack", string(debug.Stack()))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}
